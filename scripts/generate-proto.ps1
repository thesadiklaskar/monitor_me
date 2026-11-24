$ErrorActionPreference = "Stop"

$repoRoot = (Resolve-Path (Join-Path $PSScriptRoot "..")).Path
$image = "golang:1.27.4-alpine3.23"

Write-Host "Generating backend and agent gRPC sources from proto/monitor/v1/agent.proto..."
$dockerArgs = @(
    "run", "--rm",
    "--mount", "type=bind,source=$repoRoot,target=/workspace",
    "--workdir", "/workspace",
    $image,
    "sh", "scripts/generate-proto-container.sh"
)
& docker @dockerArgs
if ($LASTEXITCODE -ne 0) {
    throw "Protocol Buffer source generation failed."
}

Write-Host "Generated Go gRPC sources in backend/gen and agent/gen."
