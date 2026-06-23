#!/bin/sh
# ByteBay — déploiement complet sur l'hôte NAS
# Usage: sudo ./deploy/install.sh
set -e

ROOT="$(cd "$(dirname "$0")/.." && pwd)"
cd "$ROOT"

RED='\033[0;31m'
GREEN='\033[0;32m'
NC='\033[0m'
info()  { printf "${GREEN}==>${NC} %s\n" "$*"; }
warn()  { printf "${RED}!!${NC} %s\n" "$*"; }

if [ "$(id -u)" -ne 0 ]; then
  warn "Ce script doit être exécuté en root: sudo $0"
  exit 1
fi

# --- Dépendances hôte ---
info "Vérification des dépendances…"
MISSING=""
for bin in docker go mdadm smartctl; do
  command -v "$bin" >/dev/null 2>&1 || MISSING="$MISSING $bin"
done
if [ -n "$MISSING" ]; then
  info "Installation des paquets manquants:$MISSING"
  apt-get update -qq
  apt-get install -y -qq docker.io docker-compose-v2 smartmontools mdadm curl netplan.io \
    || apt-get install -y -qq docker.io docker-compose smartmontools mdadm curl
  command -v go >/dev/null 2>&1 || apt-get install -y -qq golang-go
fi

systemctl enable --now docker 2>/dev/null || true

# --- Groupe socket partagé hôte ↔ Docker ---
if ! getent group bytebay >/dev/null; then
  groupadd --system bytebay
fi
BYTEBAY_GID=$(getent group bytebay | cut -d: -f3)

# --- Répertoires ---
DATA_PATH="${BYTEBAY_DATA_PATH:-/srv/bytebay}"
VOLUMES_PATH="${BYTEBAY_VOLUMES_PATH:-/srv/bytebay-volumes}"
info "Répertoire données: $DATA_PATH"
info "Volumes NAS: $VOLUMES_PATH"
mkdir -p /run/bytebay /etc/bytebay /var/lib/bytebay "$DATA_PATH/public" "$DATA_PATH/ftp" "$VOLUMES_PATH"
chown root:bytebay /run/bytebay
chmod 775 /run/bytebay
chown -R root:root "$DATA_PATH"
chmod -R 755 "$DATA_PATH"

# --- Fichier .env ---
if [ ! -f .env ]; then
  cp .env.example .env
  SECRET=$(openssl rand -hex 16 2>/dev/null || head -c 16 /dev/urandom | od -An -tx1 | tr -d ' ')
  sed -i "s/change-me-in-production/$SECRET/" .env
  sed -i "s/change-me/$SECRET/" .env
  info ".env créé — changez BYTEBAY_ADMIN_PASSWORD"
fi
grep -q '^BYTEBAY_GID=' .env 2>/dev/null && \
  sed -i "s/^BYTEBAY_GID=.*/BYTEBAY_GID=$BYTEBAY_GID/" .env || \
  echo "BYTEBAY_GID=$BYTEBAY_GID" >> .env
grep -q '^BYTEBAY_DATA_PATH=' .env 2>/dev/null && \
  sed -i "s|^BYTEBAY_DATA_PATH=.*|BYTEBAY_DATA_PATH=$DATA_PATH|" .env || \
  echo "BYTEBAY_DATA_PATH=$DATA_PATH" >> .env
grep -q '^BYTEBAY_VOLUMES_PATH=' .env 2>/dev/null && \
  sed -i "s|^BYTEBAY_VOLUMES_PATH=.*|BYTEBAY_VOLUMES_PATH=$VOLUMES_PATH|" .env || \
  echo "BYTEBAY_VOLUMES_PATH=$VOLUMES_PATH" >> .env

# --- Agent hôte (mdadm + SMART uniquement) ---
info "Compilation et installation de bytebay-agent…"
(cd agent && go build -ldflags="-s -w" -o bytebay-agent ./cmd/bytebay-agent)
install -m755 agent/bytebay-agent /usr/local/bin/bytebay-agent

cp -n deploy/bytebay-agent.env.example /etc/bytebay/agent.env 2>/dev/null || true
grep -q BYTEBAY_SOCKET_GROUP /etc/bytebay/agent.env 2>/dev/null || \
  echo "BYTEBAY_SOCKET_GROUP=bytebay" >> /etc/bytebay/agent.env

cp deploy/systemd/bytebay-agent.service /etc/systemd/system/
systemctl daemon-reload
systemctl enable bytebay-agent
systemctl restart bytebay-agent

if [ ! -S /run/bytebay/agent.sock ]; then
  warn "Socket agent absent — vérifiez: journalctl -u bytebay-agent -n 20"
else
  info "Agent hôte OK: /run/bytebay/agent.sock"
fi

# --- Conteneurs Docker (engine + web) ---
info "Build et démarrage Docker (engine + web)…"
export BYTEBAY_DATA_PATH="$DATA_PATH"
docker compose build
docker compose up -d

sleep 3

# --- Vérifications ---
info "Vérification des services…"
AGENT_OK=0
ENGINE_OK=0
WEB_OK=0

curl -sf --unix-socket /run/bytebay/agent.sock http://localhost/health >/dev/null && AGENT_OK=1
curl -sf --unix-socket /run/bytebay/engine.sock http://localhost/health >/dev/null && ENGINE_OK=1
curl -sf http://localhost:8080/api/v1/health >/dev/null && WEB_OK=1

echo ""
echo "════════════════════════════════════════"
echo " ByteBay déployé"
echo "════════════════════════════════════════"
echo " Panel web  : http://$(hostname -I | awk '{print $1}'):8080"
echo " Données    : $DATA_PATH → /data (engine)"
echo " Agent      : $([ $AGENT_OK -eq 1 ] && echo OK || echo ÉCHEC) (RAID, SMART)"
echo " Engine     : $([ $ENGINE_OK -eq 1 ] && echo OK || echo ÉCHEC) (NFS, Samba, FTP)"
echo " Web API    : $([ $WEB_OK -eq 1 ] && echo OK || echo ÉCHEC)"
echo ""
echo " Sockets Unix dans /run/bytebay/:"
echo "   agent.sock  — hôte (mdadm)"
echo "   engine.sock — conteneur (partages)"
echo ""
echo " Commandes utiles:"
echo "   journalctl -u bytebay-agent -f"
echo "   docker compose -f $ROOT/docker-compose.yml logs -f"
echo "════════════════════════════════════════"

[ $AGENT_OK -eq 1 ] && [ $ENGINE_OK -eq 1 ] && [ $WEB_OK -eq 1 ] || exit 1
