<#
.SYNOPSIS
    Start the LENA2 backend (Docker Compose) and launch the Flutter mobile client on the Android emulator.

.DESCRIPTION
    1. Starts/ensures the Docker Compose stack is running (db, migrate, seed, api, web, caddy).
    2. Waits for the API /health endpoint to respond.
    3. Launches the configured Android emulator if it is not already running.
    4. Reads LENA_GOOGLE_CLIENT_ID / NEXT_PUBLIC_GOOGLE_CLIENT_ID from the repo .env.
    5. Runs `flutter run` for clients/mobile with the correct API and Google client IDs for the emulator.

.PARAMETER Emulator
    Name of the Android emulator to launch (default: Pixel_10_Pro).

.PARAMETER ApiUrl
    GraphQL endpoint the mobile client should use.
    Default for the Android emulator loopback is http://10.0.2.2:80/graphql.

.EXAMPLE
    .\scripts\run-local.ps1
    .\scripts\run-local.ps1 -Emulator Pixel_10_Pro -ApiUrl http://10.0.2.2:8080/graphql
#>
[CmdletBinding()]
param (
    [string]$Emulator = 'Pixel_10_Pro',
    [string]$ApiUrl = 'http://10.0.2.2:80/graphql'
)

$ErrorActionPreference = 'Stop'

$RepoRoot = (Resolve-Path (Join-Path $PSScriptRoot '..')).Path
$MobileDir = Join-Path $RepoRoot 'clients' 'mobile'
$EnvFile = Join-Path $RepoRoot '.env'

# Load .env into the current process so values can be passed to Flutter.
if (Test-Path $EnvFile) {
    Get-Content $EnvFile | ForEach-Object {
        if ($_ -match '^\s*([^#=\s][^=]*)=(.*)$') {
            [Environment]::SetEnvironmentVariable($matches[1].Trim(), $matches[2].Trim(), 'Process')
        }
    }
}

$GoogleClientId = if ($env:LENA_GOOGLE_CLIENT_ID) {
    $env:LENA_GOOGLE_CLIENT_ID
} elseif ($env:NEXT_PUBLIC_GOOGLE_CLIENT_ID) {
    $env:NEXT_PUBLIC_GOOGLE_CLIENT_ID
} else {
    ''
}

Push-Location $RepoRoot
try {
    Write-Host 'Starting Docker Compose stack...'
    docker compose up -d --build

    Write-Host 'Waiting for API to be healthy...'
    $health = $null
    do {
        Start-Sleep -Seconds 1
        try {
            $health = Invoke-RestMethod -Uri 'http://localhost/health' -ErrorAction SilentlyContinue
        } catch {
            $health = $null
        }
    } while ($health.status -ne 'ok')
    Write-Host 'API is healthy.'

    Write-Host 'Checking for a running Android emulator...'
    $emulatorDevice = $null
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
