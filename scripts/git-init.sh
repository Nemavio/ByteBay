#!/bin/sh
# Initialise un dépôt Git propre pour publication GitHub.
# N'inclut JAMAIS .env, .cursor, secrets ni binaires.
set -e
ROOT="$(cd "$(dirname "$0")/.." && pwd)"
cd "$ROOT"

if ! command -v git >/dev/null 2>&1; then
  echo "Erreur: installez git (sudo apt install git)"
  exit 1
fi

# Vérifier qu'aucun secret ne serait ajouté
if git check-ignore -q .env 2>/dev/null || [ -f .gitignore ]; then
  :
fi
if [ -f .env ] && ! grep -q '^\.env$' .gitignore 2>/dev/null; then
  echo "Erreur: .env doit être dans .gitignore"
  exit 1
fi

if [ -d .git ]; then
  echo "Dépôt git existant. Pour repartir de zéro (historique propre) :"
  echo "  rm -rf .git && $0"
  exit 1
fi

git init -b main

git add .gitignore .env.example LICENSE README.md AGENTS.md Makefile
git add docs/ deploy/ scripts/ agent/ engine/ web/ docker-compose.yml
git add -u 2>/dev/null || true

# Refuser explicitement les fichiers sensibles / locaux
git reset HEAD .env .cursor 2>/dev/null || true
git clean -fd --dry-run | grep -E '\.env$|\.cursor' && echo "Attention: fichiers locaux détectés" || true

if git diff --cached --quiet; then
  echo "Rien à committer."
  exit 1
fi

git commit -m "$(cat <<'EOF'
Initial public release of ByteBay.

DIY NAS panel for ARM (Kobol Helios64): host agent for RAID/SMART/network,
Docker engine for NFS/Samba/FTP, web UI with unified users and folder ACLs.
EOF
)" -c "user.name=${GIT_AUTHOR_NAME:-ByteBay}" -c "user.email=${GIT_AUTHOR_EMAIL:-bytebay@users.noreply.github.com}"

echo ""
echo "✓ Dépôt initialisé ($(git rev-list --count HEAD) commit(s))"
echo "  .env et .cursor sont ignorés et absents de l'historique."
echo ""
echo "Prochaines étapes :"
echo "  git remote add origin https://github.com/YOUR_USER/ByteBay.git"
echo "  git push -u origin main"
