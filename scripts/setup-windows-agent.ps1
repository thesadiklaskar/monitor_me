param(
    [string]$GrpcAddress = "localhost:8443"
)

$ErrorActionPreference = "Stop"
$repoRoot = (Resolve-Path (Join-Path $PSScriptRoot "..")).Path
$binary = Join-Path $repoRoot ".data\windows-agent\monitor-agent.exe"

if (-not (Test-Path $binary)) {
    & (Join-Path $PSScriptRoot "build-windows-agent.ps1")
    if ($LASTEXITCODE -ne 0) {
        throw "Windows agent build failed with exit code $LASTEXITCODE."
    }
}

& (Join-Path $PSScriptRoot "run-windows-agent.ps1") -GrpcAddress $GrpcAddress
if ($LASTEXITCODE -ne 0) {
    throw "Windows agent exited with code $LASTEXITCODE."
}
