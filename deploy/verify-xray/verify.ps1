# Verifies Wisp's real Xray gRPC integration against a live Xray-core instance.
#
# It starts Xray with the test config, runs the panel pointed at Xray's API,
# creates and deletes a user, and confirms the real AddUser/RemoveUser gRPC
# calls succeed (a creation that returns 201 proves Xray accepted the client).
#
#   pwsh deploy/verify-xray/verify.ps1
#   pwsh deploy/verify-xray/verify.ps1 -XrayExe C:\path\to\xray.exe   # skip download
param([string]$XrayExe = "")

$ErrorActionPreference = "Stop"
$here = $PSScriptRoot
$work = Join-Path $env:TEMP "wisp-xray-verify"
New-Item -ItemType Directory -Force $work | Out-Null

if (-not $XrayExe) {
  $XrayExe = Join-Path $work "xray.exe"
  if (-not (Test-Path $XrayExe)) {
    Write-Host "Downloading Xray-core (windows-64)..."
    $zip = Join-Path $work "xray.zip"
    Invoke-WebRequest "https://github.com/XTLS/Xray-core/releases/latest/download/Xray-windows-64.zip" -OutFile $zip -UseBasicParsing
    Expand-Archive $zip $work -Force
  }
}

Write-Host "Starting Xray..."
$xray = Start-Process $XrayExe -ArgumentList "run", "-c", "$here\config.json" -PassThru -NoNewWindow -RedirectStandardError "$work\xray.log"
Start-Sleep 2

Write-Host "Building and starting the panel against the live Xray API..."
Push-Location (Join-Path $here "..\..")
go build -o "$work\panel.exe" ./cmd/panel
Pop-Location

$env:WISP_DB = "$work\verify.db"
$env:WISP_ADDR = "127.0.0.1:8080"
$env:WISP_XRAY_API = "127.0.0.1:10085"
$env:WISP_INBOUND_TAG = "vless-in"
$env:WISP_NODE_FLOW = ""            # plain VLESS inbound has no flow
if (Test-Path $env:WISP_DB) { Remove-Item $env:WISP_DB }
$panel = Start-Process "$work\panel.exe" -PassThru -NoNewWindow -RedirectStandardError "$work\panel.log"
Start-Sleep 2

$pass = $true
try {
  $u = Invoke-RestMethod http://127.0.0.1:8080/api/users -Method Post -ContentType application/json -Body '{"email":"livetest"}'
  Write-Host "AddUser OK  -> user $($u.email) accepted by Xray (uuid $($u.uuid))"
  $del = "http://127.0.0.1:8080/api/users/" + $u.id
  Invoke-WebRequest $del -Method Delete -UseBasicParsing | Out-Null
  Write-Host "RemoveUser OK"
} catch {
  $pass = $false
  Write-Host "FAIL: $($_.Exception.Message)"
}

Stop-Process $panel.Id -Force
Stop-Process $xray.Id -Force

if ($pass) {
  Write-Host "`nPASS: Wisp drives a real Xray-core over gRPC (AddUser + RemoveUser)."
} else {
  Write-Host "`nSee $work\panel.log and $work\xray.log for details."
  exit 1
}
