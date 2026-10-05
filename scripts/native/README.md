# Native Linux deployment

Mirai runs as two unprivileged systemd services: mirai (panel) and mirai-node (VPN core). A remote installation with --join KEY runs only the node. PostgreSQL and Nginx use host packages; deployment needs no Docker or compiler.

| Path | Purpose |
| --- | --- |
| /opt/mirai | Verified binaries, updater and VERSION; writable only by root |
| /var/lib/mirai | Panel and node runtime data |
| /etc/mirai/mirai.env | Environment and database credentials, mode 0640 |
| /root/mirai-login.txt | Initial panel address and credentials, mode 0600 |
| /var/backups/mirai | Binary and PostgreSQL update backups |

mirai-update.path processes the panel's update button; mirai-update.timer runs the automatic update policy daily. Automatic updates are off by default. The mirai update command also works on remote node installations.

The installer and updater verify Ed25519 signatures and SHA-256 checksums. Archives contain only regular files named mirai, mirai-node and VERSION. Before replacing binaries the updater stops services and backs up PostgreSQL. A failed health check triggers restoration. Database restoration errors are visible in the systemd journal and services are restarted after recovery is attempted.

With a domain, Nginx terminates HTTPS and the Let's Encrypt certificate is copied to the panel's certificate store. A Certbot deployment hook refreshes it after renewal. The local QUIC node uses the trusted public certificate when available. Configure an optional public static directory with MIRAI_CAMOUFLAGE_DIR; directory listings and paths outside that directory are never served.

Default limits: panel GOMEMLIMIT=192MiB, MemoryMax=256M; node GOMEMLIMIT=384MiB, MemoryMax=448M. These are process budgets, not a guarantee of arbitrary client capacity. PostgreSQL, Nginx and the OS also need memory.

Supported native targets: Ubuntu 22.04+ / Debian 12+, amd64 and arm64. Releases use native-vX.Y.Z.R and the signed manifest's native.x86_64 / native.aarch64 fields.
