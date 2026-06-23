# Documentation ByteBay (français)

Bienvenue dans la documentation française de **ByteBay**.

| Guide | Description |
|-------|-------------|
| [Installation](installation.md) | Prérequis, déploiement, configuration |
| [Architecture](architecture.md) | Agent hôte, engine Docker, web |
| [Fonctionnalités](fonctionnalites.md) | RAID, montages, partages, utilisateurs, réseau |

[English documentation](../en/README.md)

## Aperçu de l'interface

![Écran de connexion ByteBay](../images/login_screen.png)

*Page de connexion au panel d'administration.*

![Fenêtres de gestion — partages NFS, utilisateurs, droits d'accès, RAID](../images/window1.png)

*Bureau multi-fenêtres : partages NFS, comptes unifiés, ACL et état RAID.*

![Tableau de bord, journaux et explorateur de fichiers](../images/window2.png)

*Tableau de bord (services, CPU, montages), journaux en direct et explorateur.*

## Résumé

ByteBay est un panneau NAS **DIY** pour usage personnel, conçu pour tourner sur un **Kobol Helios64** sous **Armbian/Debian**. L’objectif est de limiter ce qui tourne directement sur l’hôte : seul un petit agent gère le matériel ; le reste est dans Docker.

## Support

Projet hobby — pas de SLA. Ouvrez une issue GitHub pour les bugs ou idées.
