# Fonctionnalités

## Bureau web

Interface type bureau avec barre de tâches et fenêtres redimensionnables. Raccourcis clavier : **Échap** ferme la fenêtre au premier plan.

| Application | Description |
|-------------|-------------|
| Tableau de bord | État agent / engine |
| Explorateur | Arborescence + liste fichiers, upload, aperçu |
| Stockage | Inventaire disques |
| SMART | Santé disques, détail dans une fenêtre |
| RAID | Création, détail dégradé, ajout de disque |
| Montages | Formater/monter volumes vers `/volumes` |
| Partages | NFS, Samba, FTP |
| Utilisateurs | Comptes web / Samba / FTP |
| Droits d'accès | ACL par dossier |
| Paramètres réseau | netplan (IPv4/IPv6, LACP, VLAN) |

## RAID

- Niveaux : 0, 1, 5, 6, 10 via `mdadm`
- **Mode dégradé** : créer un RAID6 avec 4 emplacements et 3 disques ; ajouter le 4ᵉ plus tard
- Détails : état mdadm, slots, raisons de dégradation, progression recovery

## Montages

1. Créer le RAID (ou utiliser un disque)
2. **Montages** : formater (asynchrone avec barre de progression) et monter sous `/srv/bytebay-volumes/<nom>`
3. L’engine voit `/volumes/<nom>` sans redémarrage
4. Créer partages et ACL sur ces chemins

## Utilisateurs et droits

- Un compte = web (admin/viewer/aucun) + Samba + FTP
- **Droits d'accès** : chemins autorisés pour l’explorateur et services fichier
- Les admins web contournent les ACL

## Partages réseau

- **NFS** : export par chemin + clients IP
- **Samba** : partages CIFS
- **FTP** : vsftpd

Chemins recommandés : `/volumes/<volume>/…`

## Réseau

Configuration via **netplan** (fichier `90-bytebay.yaml`) :

- Ethernet, bonds **802.3ad (LACP)**, VLAN
- IPv4/IPv6 DHCP ou statique
- DNS global et par interface

⚠️ Une erreur peut couper l’accès réseau — gardez un accès console.

## Limites connues

- Samba : sync utilisateurs simplifiée ; ACL partages à renforcer
- FTP : utilisateurs virtuels basiques
- Pas de HTTPS intégré (mettre un reverse proxy devant si besoin)
- Projet personnel — pas de haute disponibilité cluster
