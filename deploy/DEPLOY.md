# Deploying Wisp

A production setup is one **panel** plus one or more **nodes**. The panel holds
the database, billing and dashboard; each node runs Xray + the Wisp agent.

```
panel host ──mTLS──► node host (xray + agent)
                ──► node host (xray + agent)
```

## 0. Build the binaries

On any machine with Go 1.26+:

```bash
make build      # produces ./bin/panel, ./bin/node, ./bin/wisp-certs
```

Or grab them from a [GitHub release](https://github.com/w99betaCODER/Wisp/releases)
(static linux amd64/arm64 binaries are published on every tag).

## 1. Generate mTLS certificates (once)

```bash
./bin/wisp-certs -dir certs -host node1.example.com,node2.example.com
```

This writes `ca.crt/key`, `server.crt/key`, `client.crt/key`. Distribute:

- **panel:** `client.crt`, `client.key`, `ca.crt`
- **each node:** `server.crt`, `server.key`, `ca.crt`

Keep `ca.key` and `*.key` private; never copy `ca.key` to a node.

## 2. Set up a node (Xray + agent)

**Docker (recommended):** see [`node/docker-compose.yml`](node/docker-compose.yml).

```bash
cd deploy/node
mkdir certs && cp /path/to/{server.crt,server.key,ca.crt} certs/
# set the Reality privateKey in xray-config.json:
docker run --rm ghcr.io/xtls/xray-core x25519        # prints a key pair
docker compose up -d
```

The public key from `x25519` goes into the **panel's** `WISP_REALITY_PBK`.

**systemd:** install Xray yourself, then:

```bash
sudo useradd -r -s /usr/sbin/nologin wisp
sudo install -D bin/node /opt/wisp/node
sudo install -D deploy/node.env.example /etc/wisp/node.env   # then edit it
sudo install -D deploy/systemd/wisp-node.service /etc/systemd/system/wisp-node.service
sudo systemctl enable --now wisp-node
```

## 3. Set up the panel

```bash
sudo install -D bin/panel /opt/wisp/panel
sudo install -D deploy/panel.env.example /etc/wisp/panel.env   # then edit it
sudo install -D deploy/systemd/wisp-panel.service /etc/systemd/system/wisp-panel.service
sudo systemctl enable --now wisp-panel
```

Put the dashboard behind a reverse proxy with TLS (and your own auth — the
admin API is unauthenticated by default; see [SECURITY.md](../SECURITY.md)).

## 4. Register nodes and go

```bash
curl -X POST http://127.0.0.1:8080/api/nodes \
  -H 'Content-Type: application/json' \
  -d '{"name":"node1","address":"node1.example.com:8443"}'
```

From now on every user you create is pushed to that node automatically.

## Verifying the Xray integration

Before trusting a deployment, confirm Wisp drives a **real** Xray over gRPC:

```bash
./deploy/verify-xray/verify.sh          # Linux  (downloads Xray)
pwsh deploy/verify-xray/verify.ps1      # Windows
```

It boots a throwaway Xray, points a panel at it, and creates/deletes a user —
a green **PASS** means the real `AddUser`/`RemoveUser` gRPC calls work end to
end. See [`verify-xray/`](verify-xray/) for details.
