param(
    [string]$GrpcAddress = "localhost:8443",
    [string]$CaFile = "",
    [switch]$Rebuild
)

$ErrorActionPreference = "Stop"
$repoRoot = (Resolve-Path (Join-Path $PSScriptRoot "..")).Path
$binary = Join-Path $repoRoot ".data\windows-agent\monitor-agent.exe"

if ($Rebuild -or -not (Test-Path $binary)) {
    & (Join-Path $PSScriptRoot "build-windows-agent.ps1")
    if ($LASTEXITCODE -ne 0) {
        throw "Windows agent build failed with exit code $LASTEXITCODE."
    }
}

$runArguments = @{ GrpcAddress = $GrpcAddress }
if (-not [string]::IsNullOrWhiteSpace($CaFile)) {
    $runArguments.CaFile = $CaFile
}
& (Join-Path $PSScriptRoot "run-windows-agent.ps1") @runArguments
if ($LASTEXITCODE -ne 0) {
    throw "Windows agent exited with code $LASTEXITCODE."
}
