#!/bin/sh
# Retire .env et .cursor de tout l'historique Git (avant push public).
# Nécessite git-filter-repo (recommandé) ou filter-branch.
set -e
ROOT="$(cd "$(dirname "$0")/.." && pwd)"
cd "$ROOT"

if ! command -v git >/dev/null 2>&1; then
  echo "Erreur: git requis"
  exit 1
fi
if [ ! -d .git ]; then
  echo "Pas de dépôt .git — utilisez scripts/git-init.sh pour un dépôt neuf."
  exit 1
fi

echo "!! Sauvegardez votre dépôt avant de continuer."
echo "   Cette opération réécrit l'historique."
read -r -p "Continuer ? [y/N] " ans
case "$ans" in y|Y|yes|YES) ;; *) exit 0 ;; esac

if command -v git-filter-repo >/dev/null 2>&1; then
  git filter-repo --path .env --path .cursor --invert-paths --force
  echo "✓ Historique nettoyé avec git-filter-repo"
  exit 0
fi

echo "git-filter-repo non trouvé — utilisation de filter-branch (plus lent)…"
git filter-branch --force --index-filter \
  'git rm -rf --cached --ignore-unmatch .env .cursor' \
  --prune-empty HEAD

rm -rf .git/refs/original/
git reflog expire --expire=now --all
git gc --prune=now --aggressive

echo "✓ Historique réécrit — .env et .cursor supprimés de tous les commits"
echo "  Si déjà poussé : git push --force-with-lease origin main"
