# Contributing to Wisp

Thanks for your interest in improving Wisp! This is an early-stage project and
contributions are welcome.

## Getting started

You need [Go 1.26+](https://go.dev/dl/). No other toolchain is required — the
web dashboard is plain HTML/CSS/JS embedded with `go:embed`, so there is no
front-end build step.

```bash
git clone https://github.com/w99betaCODER/Wisp.git
cd Wisp
go run ./cmd/panel        # http://localhost:8080
```

With no `WISP_XRAY_API` set, the panel and node agent run against a no-op Xray
client — every user/stats operation is logged instead of sent to a proxy, so
you can develop the whole system on a laptop with no Xray installed.

## Before opening a PR

Run the same checks CI does:

```bash
gofmt -l .        # must print nothing
go vet ./...
go build ./...
go test ./...
```

Please:

- keep the code dependency-light and idiomatic Go;
- add a test when you change behaviour (the `store`, `enforcer` and `billing`
  packages have examples to follow);
- update the README if you add or change a config variable or an API endpoint;
- write a clear commit message describing the *why*, not just the *what*.

## Project layout

| Path | What lives there |
|---|---|
| `cmd/panel` | control-plane API + dashboard |
| `cmd/node` | node agent that drives a local Xray |
| `cmd/wisp-certs` | mTLS certificate generator |
| `internal/` | store, xray, cluster, enforcer, billing, server, … |
| `web/` | embedded dashboard assets |

## Reporting bugs

Open an issue with steps to reproduce, what you expected, and what happened.
For security issues, see [SECURITY.md](SECURITY.md) instead.
