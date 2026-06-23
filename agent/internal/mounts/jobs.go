package mounts

import (
	"bufio"
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"io"
	"os/exec"
	"strings"
	"sync"
	"time"
)

type JobStatus string

const (
	JobFormatting JobStatus = "formatting"
	JobMounting   JobStatus = "mounting"
	JobDone       JobStatus = "done"
	JobError      JobStatus = "error"
)

type Job struct {
	ID       string       `json:"id"`
	Status   JobStatus    `json:"status"`
	Progress int          `json:"progress"`
	Message  string       `json:"message"`
	Error    string       `json:"error,omitempty"`
	Mount    *MountPoint  `json:"mount,omitempty"`
	Started  string       `json:"started_at"`
	Updated  string       `json:"updated_at"`
	req      CreateRequest
}

var (
	jobsMu sync.RWMutex
	jobs   = make(map[string]*Job)
)

func StartJob(req CreateRequest) (*Job, error) {
	if err := validateCreate(req); err != nil {
		return nil, err
	}
	id, err := newJobID()
	if err != nil {
		return nil, err
	}
	now := time.Now().UTC().Format(time.RFC3339)
	job := &Job{
		ID:       id,
		Status:   JobFormatting,
		Progress: 0,
		Message:  "Démarrage du formatage…",
		Started:  now,
		Updated:  now,
		req:      req,
	}
	jobsMu.Lock()
	jobs[id] = job
	jobsMu.Unlock()
	go runJob(job)
	return job.public(), nil
}

func ListActiveJobs() []*Job {
	jobsMu.RLock()
	defer jobsMu.RUnlock()
	var out []*Job
	for _, j := range jobs {
		if j.Status == JobDone || j.Status == JobError {
			continue
		}
		out = append(out, j.public())
	}
	return out
}

func GetJob(id string) (*Job, error) {
	jobsMu.RLock()
	job, ok := jobs[id]
	jobsMu.RUnlock()
	if !ok {
		return nil, fmt.Errorf("job not found")
	}
	return job.public(), nil
}

func (j *Job) public() *Job {
	return &Job{
		ID:       j.ID,
		Status:   j.Status,
		Progress: j.Progress,
		Message:  j.Message,
		Error:    j.Error,
		Mount:    j.Mount,
		Started:  j.Started,
		Updated:  j.Updated,
	}
}

func (j *Job) set(status JobStatus, progress int, message string) {
	jobsMu.Lock()
	defer jobsMu.Unlock()
	j.Status = status
	j.Progress = progress
	j.Message = message
	j.Updated = time.Now().UTC().Format(time.RFC3339)
}

func (j *Job) fail(err error) {
	jobsMu.Lock()
	defer jobsMu.Unlock()
	j.Status = JobError
	j.Error = err.Error()
	j.Message = "Échec"
	j.Updated = time.Now().UTC().Format(time.RFC3339)
}

func runJob(j *Job) {
	req := j.req
	req.Format = false

	if err := formatWithProgress(j, normalizeSource(req.Source), req.Fstype); err != nil {
		j.fail(err)
		return
	}

	j.set(JobMounting, 95, "Montage du volume…")
	mp, err := createMountOnly(req)
	if err != nil {
		j.fail(err)
		return
	}
	jobsMu.Lock()
	j.Mount = mp
	j.Status = JobDone
	j.Progress = 100
	j.Message = fmt.Sprintf("Volume %s prêt", mp.Name)
	j.Updated = time.Now().UTC().Format(time.RFC3339)
	jobsMu.Unlock()

	time.AfterFunc(30*time.Minute, func() {
		jobsMu.Lock()
		delete(jobs, j.ID)
		jobsMu.Unlock()
	})
}

func formatWithProgress(j *Job, device, fstype string) error {
	cmd, err := formatCommand(device, fstype, true)
	if err != nil {
		return err
	}
	stderr, err := cmd.StderrPipe()
	if err != nil {
		return err
	}
	if err := cmd.Start(); err != nil {
		return err
	}
	scan := bufio.NewScanner(stderr)
	for scan.Scan() {
		updateFormatProgress(j, fstype, scan.Text())
	}
	_ = drainReader(stderr)
	if err := cmd.Wait(); err != nil {
		return fmt.Errorf("mkfs: %w", err)
	}
	j.set(JobFormatting, 90, "Formatage terminé")
	return nil
}

func updateFormatProgress(j *Job, fstype, line string) {
	line = strings.TrimSpace(line)
	if line == "" {
		return
	}
	lower := strings.ToLower(line)

	switch fstype {
	case "ext4":
		switch {
		case strings.Contains(lower, "creating filesystem"):
			j.set(JobFormatting, 10, "Création du système de fichiers…")
		case strings.Contains(lower, "allocating group tables"):
			j.set(JobFormatting, 30, "Tables de groupes…")
		case strings.Contains(lower, "writing inode tables"):
			j.set(JobFormatting, 50, "Tables d'inodes…")
		case strings.Contains(lower, "creating journal"):
			j.set(JobFormatting, 70, "Journal…")
		case strings.Contains(lower, "writing superblocks"):
			j.set(JobFormatting, 85, "Superblocs…")
		case strings.Contains(lower, "done"):
			j.set(JobFormatting, 88, line)
		default:
			if len(line) < 80 {
				j.set(JobFormatting, j.Progress, line)
			}
		}
	case "xfs":
		if strings.Contains(lower, "%") {
			if p := parsePercent(line); p >= 0 {
				j.set(JobFormatting, clamp(5+p*80/100, 5, 88), line)
				return
			}
		}
		switch {
		case strings.Contains(lower, "meta-data"):
			j.set(JobFormatting, 20, "Métadonnées…")
		case strings.Contains(lower, "data blocks"):
			j.set(JobFormatting, 50, "Blocs de données…")
		case strings.Contains(lower, "realtime"):
			j.set(JobFormatting, 75, "Segments realtime…")
		default:
			if len(line) < 80 {
				j.set(JobFormatting, max(j.Progress, 15), line)
			}
		}
	case "btrfs":
		switch {
		case strings.Contains(lower, "creating"):
			j.set(JobFormatting, 25, "Création BTRFS…")
		case strings.Contains(lower, "chunk"):
			j.set(JobFormatting, 55, "Chunks…")
		case strings.Contains(lower, "block group"):
			j.set(JobFormatting, 70, "Groupes de blocs…")
		default:
			if len(line) < 80 {
				j.set(JobFormatting, max(j.Progress, 10), line)
			}
		}
	}
}

func parsePercent(line string) int {
	i := strings.Index(line, "%")
	if i < 1 {
		return -1
	}
	j := i - 1
	for j >= 0 && line[j] >= '0' && line[j] <= '9' {
		j--
	}
	n := strings.TrimSpace(line[j+1 : i])
	if n == "" {
		return -1
	}
	var p int
	fmt.Sscanf(n, "%d", &p)
	return p
}

func formatCommand(device, fstype string, verbose bool) (*exec.Cmd, error) {
	switch fstype {
	case "ext4":
		args := []string{"-F"}
		if verbose {
			args = append(args, "-v")
		}
		args = append(args, device)
		return exec.Command("mkfs.ext4", args...), nil
	case "xfs":
		args := []string{"-f"}
		if verbose {
			args = append(args, "-v")
		}
		args = append(args, device)
		return exec.Command("mkfs.xfs", args...), nil
	case "btrfs":
		return exec.Command("mkfs.btrfs", "-f", device), nil
	default:
		return nil, fmt.Errorf("unsupported fstype for format: %s", fstype)
	}
}

func drainReader(r io.Reader) error {
	_, err := io.Copy(io.Discard, r)
	return err
}

func newJobID() (string, error) {
	b := make([]byte, 8)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return hex.EncodeToString(b), nil
}

func clamp(v, lo, hi int) int {
	if v < lo {
		return lo
	}
	if v > hi {
		return hi
	}
	return v
}

func max(a, b int) int {
	if a > b {
		return a
	}
	return b
}
