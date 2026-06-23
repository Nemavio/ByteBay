# Build engine binary
FROM golang:1.22-bookworm AS build
WORKDIR /src
COPY go.mod ./
COPY . .
RUN CGO_ENABLED=0 go build -ldflags="-s -w" -o bytebay-engine ./cmd/bytebay-engine

# Runtime: Samba + NFS + FTP + supervisor
FROM debian:bookworm-slim

RUN apt-get update && apt-get install -y --no-install-recommends \
    samba \
    nfs-ganesha \
    nfs-ganesha-vfs \
    nfs-common \
    rpcbind \
    dbus \
    vsftpd \
    supervisor \
    curl \
    acl \
    iproute2 \
    && rm -rf /var/lib/apt/lists/*

COPY --from=build /src/bytebay-engine /usr/local/bin/bytebay-engine
COPY supervisord.conf /etc/supervisor/supervisord.conf
COPY smb.conf /etc/bytebay/smb.conf
COPY vsftpd.conf /etc/bytebay/vsftpd.conf
COPY vsftpd-ipv6.conf /etc/bytebay/vsftpd-ipv6.conf
COPY start-vsftpd-ipv6.sh /etc/bytebay/start-vsftpd-ipv6.sh
COPY pam-vsftpd /etc/pam.d/vsftpd
COPY entrypoint.sh /entrypoint.sh
COPY ganesha.min.conf /etc/bytebay/ganesha.min.conf
RUN chmod +x /entrypoint.sh /etc/bytebay/start-vsftpd-ipv6.sh \
    && mkdir -p /run/bytebay /var/lib/bytebay/shares /etc/samba/smb.conf.d /etc/vsftpd.d \
    /var/lib/ganesha /run/ganesha

ENV BYTEBAY_ENGINE_SOCKET=/run/bytebay/engine.sock

EXPOSE 139 445 2049 21 21100-21110

ENTRYPOINT ["/entrypoint.sh"]
