#!/bin/sh
set -e

mkdir -p /var/bytebay/sockets /run/samba /var/lib/bytebay/shares \
  /etc/samba/smb.conf.d /etc/exports.d /etc/vsftpd.d /data

cp /etc/bytebay/smb.conf /etc/samba/smb.conf

exec /usr/bin/supervisord -c /etc/supervisor/supervisord.conf
