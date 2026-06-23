#!/bin/sh
# Déprécié — utilisez deploy/install.sh
echo "Utilisez: sudo ./deploy/install.sh"
exec "$(dirname "$0")/install.sh" "$@"
