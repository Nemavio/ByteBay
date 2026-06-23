# Architecture

## Vue d’ensemble

```
┌─────────────────────────────────────────────────────────────────┐
│  Hôte Linux (Armbian / Debian)                                  │
│                                                                 │
│  bytebay-agent (systemd, root)                                  │
│    · /run/bytebay/agent.sock                                    │
│    · mdadm, SMART, disques                                      │
│    · montages → /srv/bytebay-volumes/*                          │
│    · netplan → /etc/netplan/90-bytebay.yaml                     │
│                                                                 │
│  /run/bytebay/  ──bind──►  /var/bytebay/sockets (conteneurs)    │
│                                                                 │
│  Docker                                                         │
│  ┌──────────────────────────┐  ┌──────────────────────────┐  │
│  │ bytebay-engine           │  │ bytebay-web              │  │
│  │ · engine.sock            │  │ · FastAPI + Svelte UI    │  │
│  │ · NFS, Samba, FTP        │◄─┤ · proxy Unix sockets     │  │
│  │ · /data (config)         │  │ · SQLite utilisateurs    │  │
│  │ · /volumes (rslave)      │  └──────────────────────────┘  │
│  └──────────────────────────┘                                   │
└─────────────────────────────────────────────────────────────────┘
```

## Pourquoi cette séparation ?

- **RAID et SMART** nécessitent un accès direct au matériel → agent sur l’hôte.
- **NFS/Samba/FTP** sont plus simples à isoler et mettre à jour dans un conteneur `privileged`.
- Le **panel web** n’a pas besoin du mode privilégié ; il parle aux services via sockets Unix.

## Sockets Unix

Docker monte `/run` en tmpfs dans les conteneurs ; les sockets sont donc exposés via :

```
Hôte : /run/bytebay/agent.sock, engine.sock
Conteneur web/engine : /var/bytebay/sockets/
```

## Données

| Emplacement hôte | Dans engine | Usage |
|------------------|-------------|--------|
| `BYTEBAY_DATA_PATH` | `/data` | Config, DB embarquée partagée |
| `BYTEBAY_VOLUMES_PATH` | `/volumes` | Volumes NAS (RAID montés) |

La propagation `rslave` sur `/volumes` permet d’ajouter des montages sur l’hôte sans redémarrer Docker.

## Sécurité

- Authentification JWT sur le panel web.
- Tokens optionnels `BYTEBAY_AGENT_TOKEN` / `BYTEBAY_ENGINE_TOKEN` sur les sockets.
- ACL dossiers pour l’explorateur web et sync utilisateurs Samba/FTP.
- NFS : contrôle par plage IP (pas lié aux comptes utilisateurs).

## Composants logiciels

| Dossier | Stack |
|---------|--------|
| `agent/` | Go 1.22 |
| `engine/` | Go + Samba + nfs-kernel-server + vsftpd |
| `web/backend/` | Python FastAPI |
| `web/frontend/` | Svelte 5 + Vite |
