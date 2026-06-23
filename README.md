# ByteBay

**Lightweight DIY NAS panel for ARM boards — personal use, Docker-first.**

ByteBay is a home NAS management stack originally built for a **[Kobol Helios64](https://kobol.io/helios64/)** running **Armbian / Debian**. It keeps the host minimal (RAID, disks, SMART, network) and runs file services plus the web UI in containers.

| | |
|---|---|
| **Documentation (FR)** | [docs/fr/README.md](docs/fr/README.md) |
| **Documentation (EN)** | [docs/en/README.md](docs/en/README.md) |
| **Agent / dev notes** | [AGENTS.md](AGENTS.md) |
| **License** | [MIT](LICENSE) |

## Screenshots / Captures d'écran

### Connexion · Login

<div align="center">

![Écran de connexion ByteBay](docs/images/login_screen.png?raw=true)

*Page de connexion au panel d'administration — administration panel login.*

</div>

### Bureau · Desktop

<div align="center">

![Partages NFS, utilisateurs, droits d'accès et RAID](docs/images/window1.png?raw=true)

*Multi-fenêtres : partages NFS, comptes unifiés, ACL et état RAID.*  
*Multi-window desktop: NFS shares, unified accounts, ACLs, and RAID status.*

</div>

### Tableau de bord · Dashboard

<div align="center">

![Tableau de bord, journaux et explorateur de fichiers](docs/images/window2.png?raw=true)

*Services, CPU, montages, journaux en direct et explorateur de fichiers.*  
*Services, CPU, mounts, live logs, and file explorer.*

</div>

---

## Français

### Qu’est-ce que ByteBay ?

ByteBay est un **NAS fait maison** : un bureau web (style Synology) pour gérer stockage, RAID, partages réseau et utilisateurs sur un petit serveur ARM. Le projet vise un **usage personnel**, pas un produit enterprise.

### Philosophie

- **Hôte léger** : un agent systemd (Go) pour le matériel — `mdadm`, SMART, montages, netplan.
- **Services en Docker** : NFS, Samba, FTP et API fichiers dans le conteneur `engine`.
- **Interface web en Docker** : FastAPI + Svelte, ~128–256 Mo RAM pour les conteneurs.
- **Peu de dépendances sur l’OS** : Armbian/Debian + Docker ; pas de stack lourde sur l’hôte.

### Démarrage rapide

```bash
git clone https://github.com/YOUR_USER/ByteBay.git
cd ByteBay
cp .env.example .env   # éditer mots de passe et chemins
sudo ./deploy/install.sh
```

Ouvrir `http://<ip-du-nas>:8080` — identifiants définis dans `.env`.

### Fonctionnalités principales

- Bureau web avec fenêtres (RAID, SMART, montages, partages, utilisateurs, droits d’accès, réseau, explorateur fichiers)
- RAID mdadm (dont création dégradée 3/4 disques)
- Montages volumes → `/volumes` dans l’engine (propagation Docker)
- Utilisateurs unifiés (web / Samba / FTP) + ACL dossiers
- NFS (ACL IP), Samba, FTP
- Réseau via netplan (IPv4/IPv6, DHCP/statique, LACP, VLAN)

### Prérequis

- Linux (testé Armbian/Debian sur Helios64)
- Docker, Go (build agent), `mdadm`, `smartmontools`, `netplan`

---

## English

### What is ByteBay?

ByteBay is a **DIY NAS** control panel: a Synology-like web desktop to manage storage, RAID, network shares, and users on a small ARM server. It is meant for **personal homelab** use, not as a commercial appliance.

### Design goals

- **Thin host** : a systemd agent (Go) owns hardware — `mdadm`, SMART, volume mounts, netplan.
- **Dockerized services** : NFS, Samba, FTP, and file APIs live in the `engine` container.
- **Dockerized UI** : FastAPI + Svelte; containers capped around 128–256 MB RAM.
- **Low host coupling** : Armbian/Debian + Docker; no heavy orchestration on bare metal.

### Quick start

```bash
git clone https://github.com/YOUR_USER/ByteBay.git
cd ByteBay
cp .env.example .env   # edit passwords and paths
sudo ./deploy/install.sh
```

Browse to `http://<nas-ip>:8080` — credentials from `.env`.

### Main features

- Web desktop (RAID, SMART, mounts, shares, users, access rights, network, file explorer)
- mdadm RAID (including degraded 4-slot / 3-disk creation)
- Host mounts exposed under `/volumes` in engine (rslave bind propagation)
- Unified users (web / Samba / FTP) + folder ACLs
- NFS (IP-based), Samba, FTP
- netplan networking (IPv4/IPv6, DHCP/static, LACP bonds, VLANs)

### Requirements

- Linux (Armbian/Debian on Kobol Helios64 tested)
- Docker, Go (agent build), `mdadm`, `smartmontools`, `netplan`

---

## Architecture (short)

```
Host: bytebay-agent (systemd) → /run/bytebay/agent.sock
        RAID · SMART · mounts · network (netplan)

Docker: engine  → NFS · Samba · FTP · /volumes · engine.sock
        web     → UI · proxies both sockets
```

See [docs/fr/architecture.md](docs/fr/architecture.md) or [docs/en/architecture.md](docs/en/architecture.md).

---

## Security notice

Change default passwords in `.env` before exposing the NAS. Do not commit `.env`. Network misconfiguration via the web panel can lock you out — keep serial/SSH access.

---

## Development

```bash
make agent    # build host agent
make docker   # build engine + web images
docker compose up -d --build
```

---

## Contributing

Issues and PRs welcome. This is a hobby project; API and UI may change.
