#!/bin/sh
if ! ip -6 addr show scope global 2>/dev/null | grep -q ' inet6 '; then
  exit 0
fi
exec /usr/sbin/vsftpd /etc/vsftpd-ipv6.conf
