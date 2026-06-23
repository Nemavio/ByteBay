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
    nfs-kernel-server \
    nfs-common \
    rpcbind \
    vsftpd \
    supervisor \
    curl \
    && rm -rf /var/lib/apt/lists/*

COPY --from=build /src/bytebay-engine /usr/local/bin/bytebay-engine
COPY supervisord.conf /etc/supervisor/supervisord.conf
COPY smb.conf /etc/bytebay/smb.conf
COPY vsftpd.conf /etc/vsftpd.conf
COPY entrypoint.sh /entrypoint.sh
RUN chmod +x /entrypoint.sh \
    && mkdir -p /run/bytebay /var/lib/bytebay/shares /etc/samba/smb.conf.d /etc/exports.d /etc/vsftpd.d

ENV BYTEBAY_ENGINE_SOCKET=/run/bytebay/engine.sock

EXPOSE 139 445 2049 21 21100-21110

ENTRYPOINT ["/entrypoint.sh"]
