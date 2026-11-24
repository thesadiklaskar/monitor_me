param(
    [Parameter(Mandatory = $true)]
    [string]$ServerUrl,
    [string]$GrpcAddress = ""
)

$ErrorActionPreference = "Stop"
$ServerUrl = $ServerUrl.TrimEnd("/")
if (-not ($ServerUrl.StartsWith("https://", [StringComparison]::OrdinalIgnoreCase) -or
          $ServerUrl.StartsWith("http://localhost", [StringComparison]::OrdinalIgnoreCase) -or
          $ServerUrl.StartsWith("http://127.0.0.1", [StringComparison]::OrdinalIgnoreCase))) {
    throw "Use HTTPS for a remote control plane. HTTP is allowed only for localhost testing."
}

$principal = New-Object Security.Principal.WindowsPrincipal([Security.Principal.WindowsIdentity]::GetCurrent())
if (-not $principal.IsInRole([Security.Principal.WindowsBuiltInRole]::Administrator)) {
    throw "Run PowerShell as Administrator to install a Windows Service."
}
if ([Runtime.InteropServices.RuntimeInformation]::OSArchitecture -ne [Runtime.InteropServices.Architecture]::X64) {
    throw "This release currently supports Windows x64. The target architecture was detected automatically."
}
if ([string]::IsNullOrWhiteSpace($GrpcAddress)) {
    $authority = ([Uri]$ServerUrl).Authority
    $hostName = $authority.Split(":")[0]
    $GrpcAddress = "$($hostName):8443"
}

$temporaryDirectory = Join-Path $env:TEMP ("monitor-me-" + [Guid]::NewGuid().ToString("N"))
New-Item -ItemType Directory -Force -Path $temporaryDirectory | Out-Null
try {
    $artifactUrl = "$ServerUrl/api/agent-artifacts/windows/amd64"
    $agentDownload = Join-Path $temporaryDirectory "monitor-agent.exe"
    $artifactResponse = Invoke-WebRequest -UseBasicParsing -Uri $artifactUrl -OutFile $agentDownload -PassThru
    $expectedHash = [string]$artifactResponse.Headers["X-MonitorMe-SHA256"]
    $agentVersion = [string]$artifactResponse.Headers["X-MonitorMe-Agent-Version"]
    $actualHash = (Get-FileHash -Algorithm SHA256 -Path $agentDownload).Hash.ToLowerInvariant()
    if ([string]::IsNullOrWhiteSpace($expectedHash) -or $actualHash -ne $expectedHash.ToLowerInvariant() -or [string]::IsNullOrWhiteSpace($agentVersion)) {
        throw "Native agent download failed SHA-256 or release-version verification."
    }

    $caFile = Join-Path $temporaryDirectory "monitor-me-agent-ca.pem"
    Invoke-WebRequest -UseBasicParsing -Uri "$ServerUrl/api/agent-ca" -OutFile $caFile | Out-Null
    if (-not (Select-String -Path $caFile -Pattern "-----BEGIN CERTIFICATE-----" -Quiet)) {
        throw "The control plane did not provide a valid PEM certificate."
    }

    $programDirectory = Join-Path $env:ProgramFiles "Monitor Me"
    $dataDirectory = Join-Path $env:ProgramData "MonitorMe"
    $spoolDirectory = Join-Path $dataDirectory "spool"
    $binaryPath = Join-Path $programDirectory "monitor-agent.exe"
    $stateFile = Join-Path $dataDirectory "agent-state.json"
    $configFile = Join-Path $dataDirectory "agent.env"
    New-Item -ItemType Directory -Force -Path $programDirectory, $spoolDirectory | Out-Null

    $token = ""
    if (-not (Test-Path $stateFile)) {
        $secureToken = Read-Host "Paste the one-time Monitor Me enrollment token" -AsSecureString
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

    $service = Get-Service -Name "MonitorMeAgent" -ErrorAction SilentlyContinue
    if ($service) {
        if ($service.Status -ne "Stopped") {
            Stop-Service -Name "MonitorMeAgent" -Force
            (Get-Service -Name "MonitorMeAgent").WaitForStatus("Stopped", [TimeSpan]::FromSeconds(30))
        }
    }

    $newBinary = Join-Path $temporaryDirectory "monitor-agent.new.exe"
    Copy-Item -LiteralPath $agentDownload -Destination $newBinary -Force
    Move-Item -LiteralPath $newBinary -Destination $binaryPath -Force
    Copy-Item -LiteralPath $caFile -Destination (Join-Path $dataDirectory "server-ca.pem") -Force

    $values = [ordered]@{
        MONITOR_AGENT_VERSION = $agentVersion
        MONITOR_GRPC_ADDRESS = $GrpcAddress
        MONITOR_TLS_CA_FILE = (Join-Path $dataDirectory "server-ca.pem")
        MONITOR_COLLECTION_INTERVAL = "5s"
        MONITOR_SPOOL_DIR = $spoolDirectory
        MONITOR_AGENT_STATE_FILE = $stateFile
    }
    if (-not [string]::IsNullOrWhiteSpace($token)) {
        $values.MONITOR_ENROLLMENT_TOKEN = $token
    }
    $configLines = foreach ($entry in $values.GetEnumerator()) {
        $encodedValue = ConvertTo-Json -InputObject ([string]$entry.Value) -Compress
        "$($entry.Key)=$encodedValue"
    }
    $utf8NoBom = New-Object System.Text.UTF8Encoding($false)
    [System.IO.File]::WriteAllLines($configFile, [string[]]$configLines, $utf8NoBom)
    $token = $null
    $secureToken = $null

    & icacls.exe $dataDirectory /inheritance:r /grant:r "SYSTEM:(OI)(CI)F" "BUILTIN\Administrators:(OI)(CI)F" /T | Out-Null
    if ($LASTEXITCODE -ne 0) {
        throw "Could not secure the ProgramData agent directory ACL."
    }

    $binaryCommand = '"' + $binaryPath + '" --config-file "' + $configFile + '"'
    if (-not $service) {
        New-Service -Name "MonitorMeAgent" -DisplayName "Monitor Me Host Agent" -BinaryPathName $binaryCommand -StartupType Automatic | Out-Null
    }
    else {
        & sc.exe config MonitorMeAgent binPath= $binaryCommand start= auto | Out-Null
        if ($LASTEXITCODE -ne 0) {
            throw "Could not update the Monitor Me Windows Service command line."
        }
    }
    & sc.exe description MonitorMeAgent "Collects and securely forwards native host telemetry to Monitor Me." | Out-Null
    & sc.exe failure MonitorMeAgent reset= 86400 actions= restart/5000/restart/15000/restart/60000 | Out-Null
    Start-Service -Name "MonitorMeAgent"

    if (Test-Path $stateFile) {
        $sessionPersisted = $true
    }
    else {
        $sessionPersisted = $false
        for ($attempt = 0; $attempt -lt 30; $attempt++) {
            Start-Sleep -Seconds 1
            if (Test-Path $stateFile) {
                $sessionPersisted = $true
                break
            }
        }
    }
    if (-not $sessionPersisted) {
        throw "The agent did not persist its enrollment session. Check the Windows Application event log and confirm the target can reach $GrpcAddress."
    }

    if (Select-String -Path $configFile -Pattern "^MONITOR_ENROLLMENT_TOKEN=" -Quiet) {
        $remainingLines = Get-Content -LiteralPath $configFile | Where-Object { $_ -notmatch "^MONITOR_ENROLLMENT_TOKEN=" }
        [System.IO.File]::WriteAllLines($configFile, [string[]]$remainingLines, $utf8NoBom)
        Restart-Service -Name "MonitorMeAgent"
    }

    Write-Host "Monitor Me agent installed and running as a Windows Service."
    Write-Host "Detected: Windows $([Runtime.InteropServices.RuntimeInformation]::OSArchitecture) · agent version $agentVersion"
    Write-Host "Status: Get-Service MonitorMeAgent"
    Write-Host "Logs: Get-WinEvent -LogName Application -MaxEvents 50"
}
finally {
    Remove-Item -LiteralPath $temporaryDirectory -Recurse -Force -ErrorAction SilentlyContinue
    $token = $null
}
