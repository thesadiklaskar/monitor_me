$ErrorActionPreference = "Stop"

$repoRoot = (Resolve-Path (Join-Path $PSScriptRoot "..")).Path
$image = "golang:1.27.4-alpine3.23"

Write-Host "Generating backend and agent gRPC sources from proto/monitor/v1/agent.proto..."
$dockerArgs = @(
    "run", "--rm",
    "--mount", "type=bind,source=$repoRoot,target=/workspace",
    "--workdir", "/workspace",
    $image,
    "sh", "-lc",
    'apk add --no-cache protobuf && go install google.golang.org/protobuf/cmd/protoc-gen-go@v1.36.12 && go install google.golang.org/grpc/cmd/protoc-gen-go-grpc@v1.6.2 && export PATH="$(go env GOPATH)/bin:$PATH" && sh scripts/generate-proto.sh'
)
& docker @dockerArgs
if ($LASTEXITCODE -ne 0) {
    throw "Protocol Buffer source generation failed."
}

Write-Host "Generated Go gRPC sources in backend/gen and agent/gen."
