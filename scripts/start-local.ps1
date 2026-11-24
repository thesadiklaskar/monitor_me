param(
    [switch]$NoBrowser
)

$ErrorActionPreference = "Stop"
$repoRoot = (Resolve-Path (Join-Path $PSScriptRoot "..")).Path
Set-Location $repoRoot

function Assert-CurrentSource {
    if ($env:MONITOR_ME_SKIP_SOURCE_CHECK -eq "1") {
        Write-Warning "Git source freshness check was explicitly skipped."
        return
    }
    if (-not (Get-Command git -ErrorAction SilentlyContinue)) {
        Write-Warning "Git is unavailable; source freshness cannot be verified."
        return
    }

    $insideWorkTree = & git rev-parse --is-inside-work-tree 2>$null
    if ($LASTEXITCODE -ne 0 -or "$insideWorkTree".Trim() -ne "true") {
        Write-Warning "No Git checkout detected; building the supplied source snapshot."
        return
    }

    $branch = (& git branch --show-current 2>$null).Trim()
    if ($LASTEXITCODE -ne 0 -or $branch -ne "main") {
        throw "Monitor Me development builds must run from main. Current branch: '$branch'. No containers were changed."
    }

    $localHead = (& git rev-parse --verify HEAD 2>$null).Trim()
    if ($LASTEXITCODE -ne 0) {
        throw "Could not identify the local source revision."
    }

    $originUrl = & git remote get-url origin 2>$null
    $originExitCode = $LASTEXITCODE
    if ($originExitCode -ne 0 -or -not $originUrl) {
        Write-Warning "No origin remote is configured; source freshness cannot be checked."
        Write-Host "Local source revision: $localHead"
        return
    }

    & git fetch --quiet --no-tags origin main
    if ($LASTEXITCODE -ne 0) {
        throw "Could not verify origin/main. Check GitHub access, then run 'git pull --ff-only origin main'. No containers were changed. Set MONITOR_ME_SKIP_SOURCE_CHECK=1 only for an intentional offline source snapshot."
    }

    $remoteHead = (& git rev-parse --verify FETCH_HEAD 2>$null).Trim()
    if ($LASTEXITCODE -ne 0 -or -not $remoteHead) {
        throw "Could not read the fetched origin/main revision."
    }

    Write-Host "Local source revision:  $localHead"
    Write-Host "Remote main revision:   $remoteHead"
    if ($localHead -ne $remoteHead) {
        & git merge-base --is-ancestor HEAD FETCH_HEAD 2>$null
        $localIsBehind = $LASTEXITCODE -eq 0
        if ($localIsBehind) {
            throw "Local main is behind origin/main. Run 'git pull --ff-only origin main', inspect any local changes, then retry. No Docker services were changed."
        }

        & git merge-base --is-ancestor FETCH_HEAD HEAD 2>$null
        if ($LASTEXITCODE -ne 0) {
            throw "Local main and origin/main have diverged. Do not reset or discard work; reconcile the source history before building. No Docker services were changed."
        }
        Write-Warning "Local main contains commits ahead of origin/main; building the current local checkout."
    }

    $dirtyFiles = @(& git status --short --untracked-files=all 2>$null)
    if ($LASTEXITCODE -eq 0 -and $dirtyFiles.Count -gt 0) {
        $generatedPaths = @(
            "backend/gen/monitor/v1/agent.pb.go",
            "backend/gen/monitor/v1/agent_grpc.pb.go",
            "backend/go.mod",
            "backend/go.sum",
            "web/tsconfig.tsbuildinfo"
        )
        $generatedChanges = @()
        $sourceChanges = @()
        foreach ($entry in $dirtyFiles) {
            if ([string]::IsNullOrWhiteSpace($entry) -or $entry.Length -lt 4) { continue }
            $path = $entry.Substring(3).Trim().Replace("\\", "/")
            if ($generatedPaths -contains $path) {
                $generatedChanges += $path
            } else {
                $sourceChanges += $entry
            }
        }

        if ($sourceChanges.Count -gt 0 -and $env:MONITOR_ME_ALLOW_DIRTY_SOURCE -ne "1") {
            $details = $sourceChanges -join [Environment]::NewLine
            throw ("Uncommitted source changes were found. They have not been changed or discarded:" + [Environment]::NewLine + $details + [Environment]::NewLine + "Review and preserve these files, then commit/stash before building. Set MONITOR_ME_ALLOW_DIRTY_SOURCE=1 only for an intentional modified-source build. No Docker services were changed.")
        }
        if ($generatedChanges.Count -gt 0) {
            Write-Warning ("Preserving local generated build outputs: " + (($generatedChanges | Sort-Object -Unique) -join ", "))
        }
        if ($sourceChanges.Count -gt 0 -and $env:MONITOR_ME_ALLOW_DIRTY_SOURCE -eq "1") {
            Write-Warning "Explicit modified-source override is enabled; the Docker build will include local source modifications."
        }
    }
}

Assert-CurrentSource

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
    docker run --rm --entrypoint /bin/sh --mount "type=bind,source=$certificateDirectory,target=/certs" alpine:3.23 -c "apk add --no-cache openssl >/dev/null && openssl req -x509 -newkey rsa:2048 -nodes -keyout /certs/server.key -out /certs/server.crt -days 365 -subj '/CN=localhost' -addext 'subjectAltName=DNS:localhost,IP:127.0.0.1' && chmod 644 /certs/server.crt && chmod 600 /certs/server.key"
    if ($LASTEXITCODE -ne 0) {
        throw "Local certificate generation failed."
    }
}

Write-Host "Validating Docker Compose configuration..."
docker compose config --quiet
if ($LASTEXITCODE -ne 0) {
    throw "Docker Compose configuration is invalid. No services were started or replaced."
}

Write-Host "Building every application image before changing running services..."
docker compose build
$buildExitCode = $LASTEXITCODE
if ($buildExitCode -ne 0) {
    Write-Host "Image build failed. Docker Compose was not asked to start or replace any service." -ForegroundColor Red
    Write-Host "Existing container state (may belong to an older source revision):" -ForegroundColor Yellow
    docker compose ps --all
    throw "Monitor Me image build failed (exit code $buildExitCode). Fix the build error; existing container logs are not evidence for this failed build."
}

Write-Host "All images built successfully. Starting the stack without rebuilding..."
docker compose up -d --no-build
$upExitCode = $LASTEXITCODE
if ($upExitCode -ne 0) {
    Write-Host "Docker Compose could not start the freshly built stack." -ForegroundColor Red
    docker compose ps --all
    docker compose logs --no-color --since=5m --tail=100
    throw "Docker Compose startup failed (exit code $upExitCode)."
}

Write-Host "Waiting for API and dashboard readiness..."
$ready = $false
for ($attempt = 0; $attempt -lt 40; $attempt++) {
    $apiReady = $false
    $webReady = $false
    try {
        $apiResponse = Invoke-WebRequest -UseBasicParsing -Uri "http://localhost:8080/health/ready" -TimeoutSec 3
        $apiReady = $apiResponse.StatusCode -eq 200
    } catch {}
    try {
        $webResponse = Invoke-WebRequest -UseBasicParsing -Uri "http://localhost:3000/api/config" -TimeoutSec 3
        $webReady = $webResponse.StatusCode -eq 200
    } catch {}
    if ($apiReady -and $webReady) {
        $ready = $true
        break
    }
    Start-Sleep -Seconds 2
}

if (-not $ready) {
    Write-Host "The stack did not become healthy. Recent service state:" -ForegroundColor Red
    docker compose ps --all
    docker compose logs --no-color --since=5m --tail=100
    throw "Monitor Me did not pass API and dashboard readiness checks."
}

Write-Host "Monitor Me is ready at http://localhost:3000"
if (-not $NoBrowser) { Start-Process "http://localhost:3000" }
