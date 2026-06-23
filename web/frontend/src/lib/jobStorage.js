const RAID_JOB_KEY = 'bytebay_raid_job'
const MOUNT_JOB_KEY = 'bytebay_mount_job'

export function saveRaidJob(id) {
  if (id) sessionStorage.setItem(RAID_JOB_KEY, id)
}

export function clearRaidJob() {
  sessionStorage.removeItem(RAID_JOB_KEY)
}

export function loadRaidJob() {
  return sessionStorage.getItem(RAID_JOB_KEY)
}

export function saveMountJob(id) {
  if (id) sessionStorage.setItem(MOUNT_JOB_KEY, id)
}

export function clearMountJob() {
  sessionStorage.removeItem(MOUNT_JOB_KEY)
}

export function loadMountJob() {
  return sessionStorage.getItem(MOUNT_JOB_KEY)
}
