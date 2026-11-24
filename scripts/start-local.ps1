param(
    [switch]$NoBrowser
)

$ErrorActionPreference = "Stop"
$repoRoot = (Resolve-Path (Join-Path $PSScriptRoot "..")).Path
Set-Location $repoRoot

docker info | Out-Null
if ($LASTEXITCODE -ne 0) {
    throw "Docker Engine is not running. Start Docker Desktop and retry."
}

if (-not (Test-Path ".env") -and (Test-Path ".env.example")) {
    Copy-Item ".env.example" ".env"
    Write-Host "Created .env from .env.example."
}

$certificateDirectory = Join-Path $repoRoot ".data\certs"
$certificate = Join-Path $certificateDirectory "server.crt"
$key = Join-Path $certificateDirectory "server.key"
New-Item -ItemType Directory -Force -Path $certificateDirectory | Out-Null

if (-not (Test-Path $certificate) -or -not (Test-Path $key)) {
    Write-Host "Generating a local-only development TLS certificate with Docker..."
    docker run --rm --entrypoint /bin/sh --mount "type=bind,source=$certificateDirectory,target=/certs" alpine:3.23 -c "apk add --no-cache openssl >/dev/null && openssl req -x509 -newkey rsa:2048 -nodes -keyout /certs/server.key -out /certs/server.crt -days 365 -subj '/CN=localhost' -addext 'subjectAltName=DNS:localhost,IP:127.0.0.1' && chmod 644 /certs/server.crt /certs/server.key"
    if ($LASTEXITCODE -ne 0) {
        throw "Local certificate generation failed."
    }
}

docker compose up --build -d
if ($LASTEXITCODE -ne 0) {
    throw "Docker Compose failed to start Monitor Me."
}

Write-Host "Waiting for the dashboard..."
$ready = $false
for ($attempt = 0; $attempt -lt 40; $attempt++) {
    try {
        $response = Invoke-WebRequest -Uri "http://localhost:3000/api/config" -TimeoutSec 3
        if ($response.StatusCode -eq 200) { $ready = $true; break }
    } catch {
        Start-Sleep -Seconds 2
    }
}
if (-not $ready) {
    Write-Warning "The stack is running, but the dashboard health probe did not become ready. Check: docker compose logs --tail=100"
    exit 1
}

Write-Host "Monitor Me is ready at http://localhost:3000"
if (-not $NoBrowser) { Start-Process "http://localhost:3000" }
