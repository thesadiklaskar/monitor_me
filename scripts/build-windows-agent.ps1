$ErrorActionPreference = "Stop"

$repoRoot = (Resolve-Path (Join-Path $PSScriptRoot "..")).Path
$outputDirectory = Join-Path $repoRoot ".data\windows-agent"
New-Item -ItemType Directory -Force -Path $outputDirectory | Out-Null

Write-Host "Building the Windows x64 Monitor Me agent inside Docker..."
docker run --rm --entrypoint /bin/sh --mount "type=bind,source=$repoRoot,target=/src" --workdir /src golang:1.27.4-alpine3.23 /src/scripts/build-windows-agent.sh
if ($LASTEXITCODE -ne 0) {
    throw "Windows agent build failed with exit code $LASTEXITCODE."
}

$binary = Join-Path $outputDirectory "monitor-agent.exe"
if (-not (Test-Path $binary)) {
    throw "Build finished without producing $binary."
}

Write-Host "Windows agent ready: $binary"
