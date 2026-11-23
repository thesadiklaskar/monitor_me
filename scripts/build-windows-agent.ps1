$ErrorActionPreference = "Stop"

$repoRoot = (Resolve-Path (Join-Path $PSScriptRoot "..")).Path
$outputDirectory = Join-Path $repoRoot ".data\windows-agent"
New-Item -ItemType Directory -Force -Path $outputDirectory | Out-Null

$buildCommands = @'
set -eu
apk add --no-cache protobuf
export PATH="$(go env GOPATH)/bin:$PATH"
go install google.golang.org/protobuf/cmd/protoc-gen-go@v1.36.12
go install google.golang.org/grpc/cmd/protoc-gen-go-grpc@v1.6.2
sh scripts/generate-proto.sh
cd agent
go mod tidy
GOOS=windows GOARCH=amd64 CGO_ENABLED=0 go build -trimpath -ldflags="-s -w" -o /src/.data/windows-agent/monitor-agent.exe ./cmd/monitor-agent
'@

Write-Host "Building the Windows x64 Monitor Me agent inside Docker..."
docker run --rm --entrypoint /bin/sh --mount "type=bind,source=$repoRoot,target=/src" --workdir /src golang:1.27.4-alpine3.23 -c $buildCommands
if ($LASTEXITCODE -ne 0) {
    throw "Windows agent build failed with exit code $LASTEXITCODE."
}

$binary = Join-Path $outputDirectory "monitor-agent.exe"
if (-not (Test-Path $binary)) {
    throw "Build finished without producing $binary."
}

Write-Host "Windows agent ready: $binary"
