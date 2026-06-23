#!/bin/sh
set -e

mkdir -p /var/bytebay/sockets /run/samba /var/log/samba /var/run/vsftpd/empty /var/lib/bytebay/shares \
  /etc/samba/smb.conf.d /etc/vsftpd.d /etc/vsftpd.d/bytebay-users /data /export \
  /var/lib/ganesha /run/ganesha /run/dbus

if [ ! -f /var/lib/bytebay/ganesha.conf ]; then
  cp /etc/bytebay/ganesha.min.conf /var/lib/bytebay/ganesha.conf
fi

# Désactive nfsd noyau (ancien stack) pour libérer le port 2049 à Ganesha.
if [ -w /proc/fs/nfsd/threads ] 2>/dev/null; then
  echo 0 > /proc/fs/nfsd/threads 2>/dev/null || true
fi

cp /etc/bytebay/smb.conf /etc/samba/smb.conf
cp /etc/bytebay/vsftpd.conf /etc/vsftpd.conf
cp /etc/bytebay/vsftpd-ipv6.conf /etc/vsftpd-ipv6.conf

exec /usr/bin/supervisord -c /etc/supervisor/supervisord.conf
