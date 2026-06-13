<div align="center">

# 🌫️ Wisp

**A simple, single-binary, multi-node VPN panel — with billing built in.**

Manage Xray-based VPN users across many servers from one clean dashboard.
No Python virtualenvs, no `node_modules` on your server — just one binary.

</div>

---

## Why Wisp?

[Marzban](https://github.com/Gozargah/Marzban) and [3x-ui](https://github.com/MHSanaei/3x-ui) are great, but they leave gaps:

| | 3x-ui | Marzban | **Wisp** |
|---|:---:|:---:|:---:|
| Multi-node | ❌ | ✅ | ✅ |
| Single binary deploy | ❌ | ❌ | ✅ |
| Built-in billing | ❌ | ❌ | ✅ *(planned)* |
| White-label / resellers | ❌ | ❌ | ✅ *(planned)* |
| Clean, modern UI | ⚠️ | ⚠️ | ✅ *(planned)* |

Wisp's two principles are **simplicity** and **functionality**: trivial to install,
powerful enough to run a real VPN business.

## Status

> ⚠️ **Early development.** The control-plane API and in-memory store work today.
> Xray integration, persistence, multi-node and billing are on the roadmap below.

## Quick start

Requires [Go 1.26+](https://go.dev/dl/).

```bash
git clone https://github.com/wisp-panel/wisp.git
cd wisp
go run ./cmd/panel
# panel is now on http://localhost:8080
```

Try the API:

```bash
# health check
curl http://localhost:8080/healthz

# create a user
curl -X POST http://localhost:8080/api/users \
  -H 'Content-Type: application/json' \
  -d '{"email":"alice@vpn"}'

# list users
curl http://localhost:8080/api/users
```

## API

| Method | Path | Description |
|---|---|---|
| `GET` | `/healthz` | Liveness probe |
| `GET` | `/api/users` | List all users |
| `POST` | `/api/users` | Create a user (`{"email": "..."}`) |
| `GET` | `/api/users/{id}` | Get one user |
| `DELETE` | `/api/users/{id}` | Delete a user |

## Architecture

Wisp is a **control plane / data plane** system:

```
┌─────────────────────────────────┐
│   PANEL  (control plane)         │   Go API + DB + Web UI
│   users · billing · limits       │
└───────────────┬─────────────────┘
                │ gRPC + mTLS
      ┌─────────┼─────────┐
      ▼         ▼         ▼
  ┌───────┐ ┌───────┐ ┌───────┐
  │ NODE  │ │ NODE  │ │ NODE  │       Go agent + Xray-core
  │ agent │ │ agent │ │ agent │       on each VPN server
  └───────┘ └───────┘ └───────┘
```

The panel never forwards VPN traffic itself — it tells each node's Xray which
clients to accept, and reads traffic stats back. See [`internal/`](internal/)
for the package layout.

## Roadmap

- [x] **Phase 0** — Control-plane skeleton: HTTP API, user CRUD, in-memory store
- [ ] **Phase 1** — SQLite persistence + Xray gRPC integration (VLESS + Reality), subscription links
- [ ] **Phase 2** — Multi-node: node agent, mTLS, user distribution
- [ ] **Phase 3** — Traffic limits & expiry, auto-disable
- [ ] **Phase 4** — Billing: plans, payments, auto-renewal
- [ ] **Phase 5** — White-label / resellers, web UI

## Development

```bash
make run      # start the panel
make test     # run tests
make build    # build ./bin/panel
make vet      # static checks
```

## License

[MIT](LICENSE)
