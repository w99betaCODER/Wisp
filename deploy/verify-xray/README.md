# Live Xray verification

Wisp's unit tests use a no-op Xray client. This kit checks the **real** gRPC
integration against an actual Xray-core binary, so you can be sure the
HandlerService / StatsService calls are wire-compatible with the Xray you run.

## What it does

1. starts Xray with [`config.json`](config.json) — a minimal setup with the
   gRPC **api** inbound (`HandlerService`, `StatsService`) on `127.0.0.1:10085`
   and a plain VLESS inbound tagged `vless-in`;
2. builds and starts the panel with `WISP_XRAY_API=127.0.0.1:10085`;
3. creates a user (real `AddUser` over gRPC) and deletes it (real `RemoveUser`).

A user creation that returns `201` proves Xray accepted the client — the gRPC
path works. The script prints **PASS** or **FAIL**.

## Run it

```bash
# Linux (downloads Xray-core automatically)
./verify.sh

# Windows
pwsh verify.ps1

# Use an Xray you already have
XRAY=/usr/local/bin/xray ./verify.sh
pwsh verify.ps1 -XrayExe C:\path\to\xray.exe
```

> Note: the scripts download the official Xray-core release from GitHub if you
> don't pass an existing binary. Review them first if that matters to you.
