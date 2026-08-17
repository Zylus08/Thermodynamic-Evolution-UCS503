# Run both API and frontend for local development on Windows (PowerShell)
# Launches two new PowerShell windows (one for API, one for frontend).
# This script requires ADMIN_PASSKEY to be set in the environment before running.

# Determine repository root robustly (script lives in scripts/)
$scriptDir = Split-Path -Parent $MyInvocation.MyCommand.Definition
$repoRoot = Resolve-Path -Path (Join-Path $scriptDir "..")

# Ensure ADMIN_PASSKEY is provided (no default)
if (-not $env:ADMIN_PASSKEY -or $env:ADMIN_PASSKEY -eq '') {
    Write-Host "Environment variable ADMIN_PASSKEY is not set. Please set it before running. Example (PowerShell):`n  $env:ADMIN_PASSKEY = 'your_local_passkey_here'" -ForegroundColor Red
    exit 1
}

# Paths
$apiPath = Join-Path $repoRoot 'api'
$fePath = Join-Path $repoRoot 'admin-portal'

if (-not (Test-Path $apiPath)) {
    Write-Host "API path not found: $apiPath" -ForegroundColor Red
    exit 1
}
if (-not (Test-Path $fePath)) {
    Write-Host "Frontend path not found: $fePath" -ForegroundColor Red
    exit 1
}

# Start API in new window (child process will inherit ADMIN_PASSKEY from environment)
$apiCmd = "Set-Location -Path '$apiPath'; `$env:PORT='8080'; if (Get-Command go -ErrorAction SilentlyContinue) { go run main.go } else { Write-Host 'Go not found in PATH'; Start-Sleep -Seconds 30 }"
Start-Process powershell -ArgumentList "-NoExit","-Command",$apiCmd

# Start frontend in new window
$feCmd = "Set-Location -Path '$fePath'; if (Get-Command npm -ErrorAction SilentlyContinue) { npm ci; npm run dev -- --host 0.0.0.0 --port 5173 } else { Write-Host 'npm not found in PATH'; Start-Sleep -Seconds 30 }"
Start-Process powershell -ArgumentList "-NoExit","-Command",$feCmd

Write-Host 'Started API and frontend in separate PowerShell windows.' -ForegroundColor Green
Write-Host "API: http://localhost:8080/status" -ForegroundColor Cyan
Write-Host "Frontend: http://localhost:5173" -ForegroundColor Cyan
