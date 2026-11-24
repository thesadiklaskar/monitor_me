# Linux production deployment

This directory provides a single-node production-oriented Compose topology and operator scripts. It does not prove that a production host has been provisioned or tested.

## Network layout

- Host Nginx terminates HTTPS on TCP 443.
- Nginx exposes only the Linux/Windows installer scripts, public agent CA, and checksum-verified artifact endpoints to monitored nodes.
- The dashboard, its normal API routes and /v1/live/ws are restricted by a generated administrator/VPN allowlist when the no-login profile is enabled.
- Nginx proxies /v1/live/ws to API HTTP on 127.0.0.1:18080 for browser live telemetry.
- The API exposes TLS gRPC on TCP 8443 for enrolled agents.
- PostgreSQL and Redis have no host-published ports and are reachable only on the Compose network.
- API HTTP and Next.js are loopback-bound. Do not change them to 0.0.0.0.

## Prepare and configure

Provision a supported Linux VPS/VM, DNS, Docker Engine/Compose v2, Nginx, Certbot, curl and the host firewall manager. Obtain a valid certificate for the public hostname. For the initial issue, install the temporary HTTP-only deploy/production/nginx-acme-bootstrap.conf.example as the Nginx vhost, replace the hostname and create /var/www/certbot, then run nginx -t/reload and certbot certonly --webroot -w /var/www/certbot -d $PUBLIC_HOSTNAME. After the certificate hook copies the key for the API, replace the temporary vhost with nginx.conf.example and reload Nginx. Keep a second SSH session or provider console available before any firewall change.

Copy the repository to a stable host path, then run:

    cp deploy/production/.env.production.example .env.production
    chmod 600 .env.production

Replace every placeholder with unique secrets. URL-encode reserved characters in DATABASE_URL. Set PUBLIC_HOSTNAME, AGENT_GRPC_PUBLIC_ADDRESS, and the public API origin consistently. The API runs as a non-root user: do not mount Certbot's root-only private key directly. Instead, create a dedicated host group with the configured unused numeric CERT_GROUP_ID (default 15001), add the ID to .env.production, and let the certificate hook copy the files with group-only read permissions:

    sudo groupadd --system --gid 15001 monitor-me-cert
    sudo install -d -o root -g monitor-me-cert -m 0750 /var/lib/monitor-me/certs
    sudo RENEWED_LINEAGE="/etc/letsencrypt/live/$PUBLIC_HOSTNAME" CERT_GROUP_ID=15001 AGENT_TLS_CERT_DIR=/var/lib/monitor-me/certs ./deploy/production/certbot-deploy-hook.sh
    sudo ln -s "$PWD/deploy/production/certbot-deploy-hook.sh" /etc/letsencrypt/renewal-hooks/deploy/monitor-me.sh

Check that GID 15001 is unused before creating it; if it is already allocated, select another dedicated unused GID and set the same value in .env.production and the hook. The hook copies the renewed chain/key to the API certificate directory with mode 0640 and the dedicated group, so the container can read the key without running as root. Do not commit .env.production.

## Deploy

    ./deploy/production/deploy.sh

The script validates Compose interpolation, builds version-tagged images, starts PostgreSQL/Redis, waits for readiness, applies SQL migrations, starts API/web, and probes API readiness plus the web config endpoint. It does not configure DNS, Nginx, cloud security groups, or host firewall rules.

After the initial certificate is issued and the API-readable key copy is created, replace the temporary bootstrap vhost with nginx.conf.example as /etc/nginx/conf.d/monitor-me.conf, then run nginx -t and reload Nginx. Certbot's deploy hook refreshes the API-readable group-restricted copy on subsequent renewals. Verify HTTPS, /v1/live/ws WebSocket upgrade and gRPC TLS from an independent network. Enrolled agents must trust the certificate CA used by the public gRPC endpoint.

## Firewall helper

    ./deploy/production/firewall.sh --role control-plane --admin-cidr 203.0.113.10/32
    ./deploy/production/firewall.sh --role agent-only

Dry-run is default. Control-plane mode adds source-restricted SSH, public TCP 443 for TLS, and gRPC (source-restricted when --grpc-cidr is supplied; otherwise public for roaming agents). Port 443 must reach Nginx because remote nodes fetch the public installer/CA/artifact routes there. The no-login dashboard is protected at the Nginx layer, not by closing the entire HTTPS port. The deployment script writes /etc/nginx/snippets/monitor-me-admin-allowlist.conf from ADMIN_CIDR and verifies that Nginx includes it in both the dashboard and live-WebSocket locations, while the four public bootstrap routes remain available.

You can use repeated --admin-cidr arguments for SSH administrator ranges. For a private/VPN agent fleet, repeated --grpc-cidr arguments can restrict gRPC ingress to known source ranges; omit that option only when public gRPC ingress is intended. The helper detects active UFW/firewalld and refuses to enable an inactive firewall or choose between competing managers. It does not automatically configure cloud security groups.

The helper never changes default policies, removes rules, flushes firewall state or disables a firewall. Existing broad rules and Docker/cloud forwarding paths still require external review. --rollback removes only additions recorded by this helper; it cannot revert manual or cloud security-group changes. Agent-only hosts need no inbound Monitor Me port. Restricted outbound environments need TCP gRPC to the control-plane address plus HTTPS to download installers/agent updates.

Never publish PostgreSQL 5432, Redis 6379, API 8080 or web 3000. Docker bridge NAT can bypass simplistic UFW assumptions; this topology avoids public container publishing for those services.

## Backups, restore and rollback

    ./deploy/production/backup.sh /secure/storage/monitor-me.dump
    ./deploy/production/restore.sh --confirm /secure/storage/monitor-me.dump

Backups use PostgreSQL custom format, mode 0600 and refuse to overwrite. Restore requires explicit --confirm and makes a pre-restore snapshot first. Move backups to separate storage, encrypt at rest, restrict access, and rehearse restore on an isolated staging database.

These scripts back up PostgreSQL only. Protect TLS private keys and .env.production through a separate encrypted secrets backup. Redis is not the source of truth.

Images are tagged by MONITOR_ME_RELEASE. Keep previous images on disk. Set MONITOR_ME_RELEASE in .env.production to the previous tag and run rollback.sh. It restores app images only; it does not reverse SQL migrations or restore data. Confirm schema compatibility first. Database restore is destructive and uses the explicit restore script.

## Release evidence still required

- Real host deployment and independent external IPv4/IPv6 checks.
- DNS/TLS and successful agent enrollment over gRPC.
- Independent restore rehearsal, service recovery and application rollback exercise.
- Cloud security-group and Docker firewall-path review.
- Firewall verification on supported UFW/firewalld Linux distributions.
