# ByteBay — guide agents

NAS ARM : bureau web + agent hôte + engine Docker.

## Architecture

```
┌──────────────────────────────────────────────────────────────────┐
│  Hôte Linux                                                      │
│  bytebay-agent (systemd) ── /run/bytebay/agent.sock              │
│    · mdadm RAID                                                  │
│    · SMART                                                       │
│    · inventaire disques                                          │
│                                                                  │
│  /srv/bytebay  ──monté──►  /data (engine)                        │
│                                                                  │
│  Docker                                                          │
│  ┌─────────────────────┐    ┌─────────────────────┐            │
│  │ bytebay-engine      │    │ bytebay-web         │            │
│  │ NFS Samba FTP       │    │ FastAPI + Svelte    │            │
│  │ /run/.../engine.sock│◄───│ proxy 2 sockets     │            │
│  └─────────────────────┘    └─────────────────────┘            │
└──────────────────────────────────────────────────────────────────┘
```

**RAID/SMART = hôte.** **Partages réseau = engine Docker.** Communication via sockets Unix dans `/run/bytebay/`.

## Layout

| Path | Rôle |
|------|------|
| `agent/` | Agent hôte Go (mdadm, SMART) |
| `engine/` | Engine Docker Go + Samba/NFS/FTP |
| `web/` | Panel admin (auth, proxy sockets) |
| `deploy/install.sh` | Déploiement complet hôte |

## Déploiement

```bash
sudo ./deploy/install.sh
```

## Conventions

- Partages : chemins sous `/volumes/…` dans l'engine (montés depuis l'hôte)
- API : `/api/v1`
- Secrets : `.env` local uniquement, jamais commité
- Documentation : `docs/fr/`, `docs/en/`
