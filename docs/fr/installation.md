# Installation

## Matériel cible

ByteBay a été développé et testé sur un **Kobol Helios64** (RK3399, 4× SATA). Il devrait fonctionner sur d’autres boards ARM64 avec Debian/Armbian, Docker et `mdadm`.

## Prérequis

| Composant | Rôle |
|-----------|------|
| Docker + Compose | Conteneurs `engine` et `web` |
| Go 1.22+ | Compilation de `bytebay-agent` |
| mdadm | RAID |
| smartmontools | SMART |
| netplan.io | Configuration réseau (panel Réseau) |

## Installation automatique

```bash
git clone https://github.com/YOUR_USER/ByteBay.git
cd ByteBay
cp .env.example .env
```

Éditez `.env` :

- `BYTEBAY_ADMIN_PASSWORD` — mot de passe du panneau web
- `BYTEBAY_SECRET_KEY` — clé JWT (générée par `install.sh` si absent)
- `BYTEBAY_DATA_PATH` — données système (base SQLite, etc.)
- `BYTEBAY_VOLUMES_PATH` — racine des volumes NAS montés vers l’engine

Puis :

```bash
sudo ./deploy/install.sh
```

Le script :

1. Installe les dépendances manquantes
2. Crée le groupe `bytebay` et `/run/bytebay`
3. Compile et installe `bytebay-agent` (systemd)
4. Construit les images Docker et lance `docker compose up -d`

## Accès au panneau

URL : `http://<ip>:8080` (port configurable via `BYTEBAY_WEB_PORT`).

Identifiants : `BYTEBAY_ADMIN_USER` / `BYTEBAY_ADMIN_PASSWORD` dans `.env`.

## Mise à jour

```bash
cd ByteBay
git pull
sudo ./deploy/install.sh
```

Ou manuellement :

```bash
cd agent && go build -ldflags="-s -w" -o bytebay-agent ./cmd/bytebay-agent
sudo install -m755 bytebay-agent /usr/local/bin/
sudo systemctl restart bytebay-agent
docker compose build && docker compose up -d
```

## Fichiers importants

| Chemin | Description |
|--------|-------------|
| `/etc/bytebay/agent.env` | Variables agent (optionnel) |
| `/etc/netplan/90-bytebay.yaml` | Réseau géré par le panel |
| `/var/lib/bytebay/` | État agent (montages, etc.) |
| `.env` | Secrets Docker Compose (**ne pas committer**) |

## Dépannage

- **Agent / Engine hors ligne** (tableau de bord) : vérifier les sockets sous `/run/bytebay/` et le montage Docker `/var/bytebay/sockets` (voir architecture).
- **Pas d’accès web après changement réseau** : console série ou SSH local ; corriger netplan à la main si besoin.
