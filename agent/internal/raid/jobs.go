package raid

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"strings"
	"sync"
	"time"
)

type JobStatus string

const (
	JobPreparing JobStatus = "preparing"
	JobCreating  JobStatus = "creating"
	JobSyncing   JobStatus = "syncing"
	JobDone      JobStatus = "done"
	JobError     JobStatus = "error"
)

type CreateJob struct {
	ID       string     `json:"id"`
	Status   JobStatus  `json:"status"`
	Progress int        `json:"progress"`
	Message  string     `json:"message"`
	Error    string     `json:"error,omitempty"`
	Array    *Array     `json:"array,omitempty"`
	Started  string     `json:"started_at"`
	Updated  string     `json:"updated_at"`
	plan     *createPlan
}

var (
	createJobsMu sync.RWMutex
	createJobs   = make(map[string]*CreateJob)
)

func ListActiveCreateJobs() []*CreateJob {
	createJobsMu.RLock()
	defer createJobsMu.RUnlock()
	var out []*CreateJob
	for _, j := range createJobs {
		if j.Status == JobDone || j.Status == JobError {
			continue
		}
		out = append(out, j.public())
	}
	return out
}

func StartCreateJob(req CreateRequest) (*CreateJob, error) {
	plan, err := planCreate(req)
	if err != nil {
		return nil, err
	}
	id, err := newCreateJobID()
	if err != nil {
		return nil, err
	}
	now := time.Now().UTC().Format(time.RFC3339)
	job := &CreateJob{
		ID:       id,
		Status:   JobPreparing,
		Progress: 0,
		Message:  "Préparation des disques…",
		Started:  now,
		Updated:  now,
		plan:     plan,
	}
	createJobsMu.Lock()
	createJobs[id] = job
	createJobsMu.Unlock()
	go runCreateJob(job)
	return job.public(), nil
}

func GetCreateJob(id string) (*CreateJob, error) {
	createJobsMu.RLock()
	job, ok := createJobs[id]
	createJobsMu.RUnlock()
	if !ok {
		return nil, fmt.Errorf("job not found")
	}
	return job.public(), nil
}

func (j *CreateJob) public() *CreateJob {
	return &CreateJob{
		ID:       j.ID,
		Status:   j.Status,
		Progress: j.Progress,
		Message:  j.Message,
		Error:    j.Error,
		Array:    j.Array,
		Started:  j.Started,
		Updated:  j.Updated,
	}
}

func (j *CreateJob) set(status JobStatus, progress int, message string) {
	createJobsMu.Lock()
	defer createJobsMu.Unlock()
	j.Status = status
	j.Progress = progress
	j.Message = message
	j.Updated = time.Now().UTC().Format(time.RFC3339)
}

func (j *CreateJob) fail(err error) {
	createJobsMu.Lock()
	defer createJobsMu.Unlock()
	j.Status = JobError
	j.Error = err.Error()
	j.Message = "Échec de la création RAID"
	j.Updated = time.Now().UTC().Format(time.RFC3339)
}

func runCreateJob(j *CreateJob) {
	plan := j.plan
	n := len(plan.present)
	for i, d := range plan.present {
		j.set(JobPreparing, 5+i*15/max(n, 1), fmt.Sprintf("Nettoyage de %s…", filepathBase(d)))
		if err := prepareDeviceForRaid(d); err != nil {
			j.fail(err)
			return
		}
	}

	j.set(JobCreating, 25, fmt.Sprintf("Création de %s (RAID %s)…", plan.mdPath, plan.level))
	arr, err := mdadmCreate(plan)
	if err != nil {
		j.fail(err)
		return
	}

	j.set(JobCreating, 30, fmt.Sprintf("Array %s créé", plan.mdPath))
	waitInitialSync(j, plan.mdName, arr)
}

func waitInitialSync(j *CreateJob, mdName string, arr *Array) {
	const (
		pollInterval = time.Second
		maxWait      = 30 * time.Minute
	)
	deadline := time.Now().Add(maxWait)
	idleTicks := 0

	for time.Now().Before(deadline) {
		action, pct, syncing := SyncProgress(mdName)
		if syncing {
			idleTicks = 0
			progress := 30 + int(pct*70/100)
			if progress > 99 {
				progress = 99
			}
			j.set(JobSyncing, progress, fmt.Sprintf("%s : %.1f%%", syncActionLabel(action), pct))
			if pct >= 99.95 {
				break
			}
		} else {
			idleTicks++
			if idleTicks >= 3 {
				break
			}
		}
		time.Sleep(pollInterval)
	}

	createJobsMu.Lock()
	if fresh, err := readArrayPtr(mdName); err == nil && fresh.Level != "" {
		arr = fresh
	}
	j.Array = arr
	j.Status = JobDone
	j.Progress = 100
	if arr.Degraded {
		j.Message = fmt.Sprintf("%s prêt (mode dégradé)", arr.Path)
	} else {
		j.Message = fmt.Sprintf("%s prêt", arr.Path)
	}
	j.Updated = time.Now().UTC().Format(time.RFC3339)
	createJobsMu.Unlock()

	time.AfterFunc(30*time.Minute, func() {
		createJobsMu.Lock()
		delete(createJobs, j.ID)
		createJobsMu.Unlock()
	})
}

func syncActionLabel(action string) string {
	switch strings.ToLower(action) {
	case "recovery":
		return "Récupération"
	case "resync":
		return "Synchronisation"
	case "reshape":
		return "Reshape"
	case "check":
		return "Vérification"
	default:
		if action == "" {
			return "Synchronisation"
		}
		return action
	}
}

func filepathBase(path string) string {
	if i := strings.LastIndex(path, "/"); i >= 0 {
		return path[i+1:]
	}
	return path
}

func newCreateJobID() (string, error) {
	b := make([]byte, 8)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return hex.EncodeToString(b), nil
}

func max(a, b int) int {
	if a > b {
		return a
	}
	return b
}
