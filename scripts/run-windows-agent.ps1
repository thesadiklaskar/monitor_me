param(
    [string]$GrpcAddress = "localhost:8443"
)

$ErrorActionPreference = "Stop"
$repoRoot = (Resolve-Path (Join-Path $PSScriptRoot "..")).Path
$binary = Join-Path $repoRoot ".data\windows-agent\monitor-agent.exe"
$certificate = Join-Path $repoRoot ".data\certs\server.crt"

if (-not (Test-Path $binary)) {
    throw "Windows agent binary not found. Run .\scripts\build-windows-agent.ps1 first."
}
if (-not (Test-Path $certificate)) {
    throw "Local development CA not found at $certificate. Start the local Compose setup and generate its certificate first."
}

$dataDirectory = Join-Path $env:LOCALAPPDATA "MonitorMe"
$spoolDirectory = Join-Path $dataDirectory "spool"
New-Item -ItemType Directory -Force -Path $spoolDirectory | Out-Null

$stateFile = Join-Path $dataDirectory "agent-state.json"
$token = ""
if (-not (Test-Path $stateFile)) {
    $secureToken = Read-Host "Paste the one-time enrollment token from the Monitor Me Add Server dialog" -AsSecureString
    if ($secureToken.Length -eq 0) {
        throw "An enrollment token is required for first enrollment."
    }
    $tokenPointer = [Runtime.InteropServices.Marshal]::SecureStringToBSTR($secureToken)
    try {
        $token = [Runtime.InteropServices.Marshal]::PtrToStringBSTR($tokenPointer)
    }
    finally {
        [Runtime.InteropServices.Marshal]::ZeroFreeBSTR($tokenPointer)
    }
}

$env:MONITOR_AGENT_VERSION = "0.1.0-windows-local"
$env:MONITOR_GRPC_ADDRESS = $GrpcAddress
$env:MONITOR_TLS_CA_FILE = $certificate
$env:MONITOR_COLLECTION_INTERVAL = "5s"
$env:MONITOR_SPOOL_DIR = $spoolDirectory
$env:MONITOR_AGENT_STATE_FILE = $stateFile
if (-not [string]::IsNullOrWhiteSpace($token)) {
    $env:MONITOR_ENROLLMENT_TOKEN = $token
} else {
    Remove-Item Env:MONITOR_ENROLLMENT_TOKEN -ErrorAction SilentlyContinue
}

Write-Host "Starting the Windows host agent. Press Ctrl+C to stop it."
Write-Host "Metrics are sent to $GrpcAddress every 5 seconds."
try {
    & $binary
    if ($LASTEXITCODE -ne 0) {
        throw "Windows agent exited with code $LASTEXITCODE."
    }
}
finally {
    Remove-Item Env:MONITOR_ENROLLMENT_TOKEN -ErrorAction SilentlyContinue
    Remove-Item Env:MONITOR_TLS_CA_FILE -ErrorAction SilentlyContinue
    Remove-Item Env:MONITOR_GRPC_ADDRESS -ErrorAction SilentlyContinue
    Remove-Item Env:MONITOR_COLLECTION_INTERVAL -ErrorAction SilentlyContinue
    Remove-Item Env:MONITOR_SPOOL_DIR -ErrorAction SilentlyContinue
    Remove-Item Env:MONITOR_AGENT_STATE_FILE -ErrorAction SilentlyContinue
    Remove-Item Env:MONITOR_AGENT_VERSION -ErrorAction SilentlyContinue
}
