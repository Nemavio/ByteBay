# Architecture

## Overview

```
┌─────────────────────────────────────────────────────────────────┐
│  Linux host (Armbian / Debian)                                  │
│                                                                 │
│  bytebay-agent (systemd, root)                                  │
│    · /run/bytebay/agent.sock                                    │
│    · mdadm, SMART, disks                                        │
│    · mounts → /srv/bytebay-volumes/*                            │
│    · netplan → /etc/netplan/90-bytebay.yaml                     │
│                                                                 │
│  /run/bytebay/  ──bind──►  /var/bytebay/sockets (containers)    │
│                                                                 │
│  Docker                                                         │
│  ┌──────────────────────────┐  ┌──────────────────────────┐  │
│  │ bytebay-engine           │  │ bytebay-web              │  │
│  │ · engine.sock            │  │ · FastAPI + Svelte UI    │  │
│  │ · NFS, Samba, FTP        │◄─┤ · Unix socket proxy      │  │
│  │ · /data (config)         │  │ · SQLite users           │  │
│  │ · /volumes (rslave)      │  └──────────────────────────┘  │
│  └──────────────────────────┘                                   │
└─────────────────────────────────────────────────────────────────┘
```

## Why split host and Docker?

- **RAID and SMART** need direct hardware access → host agent.
- **NFS/Samba/FTP** are easier to isolate and upgrade in a `privileged` container.
- The **web panel** does not need privileged mode; it talks to services via Unix sockets.

## Unix sockets

Docker mounts `/run` as tmpfs inside containers, so sockets are exposed as:

```
Host: /run/bytebay/agent.sock, engine.sock
web/engine containers: /var/bytebay/sockets/
```

## Data layout

| Host path | In engine | Purpose |
|-----------|-----------|---------|
| `BYTEBAY_DATA_PATH` | `/data` | Config, shared DB volume |
| `BYTEBAY_VOLUMES_PATH` | `/volumes` | NAS volumes (mounted RAID) |

`rslave` propagation on `/volumes` lets new host mounts appear in the container without restarting Docker.

## Security

- JWT authentication on the web panel.
- Optional `BYTEBAY_AGENT_TOKEN` / `BYTEBAY_ENGINE_TOKEN` on sockets.
- Folder ACLs for web explorer and Samba/FTP user sync.
- NFS: IP-based access control (not tied to user accounts).

## Software stack

| Directory | Stack |
|-----------|--------|
| `agent/` | Go 1.22 |
| `engine/` | Go + Samba + nfs-kernel-server + vsftpd |
| `web/backend/` | Python FastAPI |
| `web/frontend/` | Svelte 5 + Vite |
