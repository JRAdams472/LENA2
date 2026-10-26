<#
.SYNOPSIS
    Start the LENA2 backend (Docker Compose) and launch the Flutter mobile client on the Android emulator.

.DESCRIPTION
    1. Starts/ensures the Docker Compose stack is running (db, migrate, seed, api, web, caddy).
    2. Waits up to -HealthTimeoutSec for the API /health endpoint to respond.
    3. Launches the configured Android emulator if it is not already running
       (gives up after -EmulatorTimeoutSec).
    4. Reads only LENA_GOOGLE_CLIENT_ID / NEXT_PUBLIC_GOOGLE_CLIENT_ID from the
       repo .env — parsing the file in place rather than loading every secret
       into the process environment (where it would persist after the script
       and leak into unrelated child processes).
    5. Runs `flutter run` for clients/mobile with the correct API and Google client IDs for the emulator.

.PARAMETER Emulator
    Name of the Android emulator to launch (default: Pixel_10_Pro).

.PARAMETER ApiUrl
    GraphQL endpoint the mobile client should use.
    Default for the Android emulator loopback is http://10.0.2.2:80/graphql.

.PARAMETER HealthTimeoutSec
    Seconds to wait for the API health endpoint (default 180 — a cold
    --build can exceed a minute).

.PARAMETER EmulatorTimeoutSec
    Seconds to wait for the emulator to appear in `flutter devices`
    (default 120).

.EXAMPLE
    .\scripts\run-local.ps1
    .\scripts\run-local.ps1 -Emulator Pixel_10_Pro -ApiUrl http://10.0.2.2:8080/graphql
#>
[CmdletBinding()]
param (
    [string]$Emulator = 'Pixel_10_Pro',
    [string]$ApiUrl = 'http://10.0.2.2:80/graphql',
    [int]$HealthTimeoutSec = 180,
    [int]$EmulatorTimeoutSec = 120
)

$ErrorActionPreference = 'Stop'

$RepoRoot = (Resolve-Path (Join-Path $PSScriptRoot '..')).Path
$MobileDir = Join-Path $RepoRoot 'clients' 'mobile'
$EnvFile = Join-Path $RepoRoot '.env'

# Read just the two keys needed — the file is parsed in place so secrets
# never enter this shell's environment.
$envValues = @{}
if (Test-Path $EnvFile) {
    Get-Content $EnvFile | ForEach-Object {
        if ($_ -match '^\s*([^#=\s][^=]*)=(.*)$') {
            $envValues[$matches[1].Trim()] = $matches[2].Trim()
        }
    }
}

$GoogleClientId = if ($envValues['LENA_GOOGLE_CLIENT_ID']) {
    $envValues['LENA_GOOGLE_CLIENT_ID']
} elseif ($envValues['NEXT_PUBLIC_GOOGLE_CLIENT_ID']) {
    $envValues['NEXT_PUBLIC_GOOGLE_CLIENT_ID']
} elseif ($env:LENA_GOOGLE_CLIENT_ID) {
    $env:LENA_GOOGLE_CLIENT_ID
} else {
    ''
}

Push-Location $RepoRoot
try {
    Write-Host 'Starting Docker Compose stack...'
    docker compose up -d --build

    Write-Host "Waiting up to ${HealthTimeoutSec}s for the API to be healthy..."
    $health = $null
    $deadline = (Get-Date).AddSeconds($HealthTimeoutSec)
    do {
        Start-Sleep -Seconds 1
        try {
            $health = Invoke-RestMethod -Uri 'http://localhost/health' -ErrorAction SilentlyContinue -TimeoutSec 5
        } catch {
            $health = $null
        }
        if ((Get-Date) -ge $deadline) {
            docker compose logs --tail 50 api | Write-Host
            throw "API did not become healthy within ${HealthTimeoutSec}s — see the api log tail above."
        }
    } while ($health.status -ne 'ok')
    Write-Host 'API is healthy.'

    Write-Host 'Checking for a running Android emulator...'
    $emulatorDevice = $null
    $emulatorDeadline = (Get-Date).AddSeconds($EmulatorTimeoutSec)
    do {
        $devices = flutter devices --machine | ConvertFrom-Json
        $emulatorDevice = $devices | Where-Object { $_.id -match 'emulator' } | Select-Object -First 1
        if (-not $emulatorDevice) {
            if ($emulatorLaunched) {
                Write-Host 'Waiting for emulator to appear...'
            } else {
                Write-Host "Launching emulator $Emulator..."
                flutter emulators --launch $Emulator
                $emulatorLaunched = $true
            }
            if ((Get-Date) -ge $emulatorDeadline) {
                throw "Emulator $Emulator did not appear in 'flutter devices' within ${EmulatorTimeoutSec}s."
            }
            Start-Sleep -Seconds 2
        }
    } while (-not $emulatorDevice)

    $deviceId = $emulatorDevice.id
    Write-Host "Running mobile app on $deviceId..."

    $dartDefines = @(
        "--dart-define=LENA_API_URL=$ApiUrl"
    )
    if ($GoogleClientId) {
        $dartDefines += "--dart-define=LENA_GOOGLE_SERVER_CLIENT_ID=$GoogleClientId"
    } else {
        Write-Warning 'No LENA_GOOGLE_CLIENT_ID / NEXT_PUBLIC_GOOGLE_CLIENT_ID found in .env. Google sign-in may not return an ID token.'
    }

    Push-Location $MobileDir
    try {
        flutter run -d $deviceId @dartDefines
    } finally {
        Pop-Location
    }
} finally {
    Pop-Location
}
