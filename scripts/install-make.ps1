# Install GNU Make on Windows when missing (PowerShell). Repo: BiFrost bootstrap uses make.
# Usage: pwsh -ExecutionPolicy Bypass -File scripts/install-make.ps1
# Set $env:SKIP_AUTO_MAKE=1 to skip.
$ErrorActionPreference = "Stop"

function Test-GnuMake {
    $mk = Get-Command make -ErrorAction SilentlyContinue
    if (-not $mk) { return $false }
    $ver = & make --version 2>$null
    return ($ver -match "GNU Make")
}

function Refresh-Path {
    $machine = [Environment]::GetEnvironmentVariable("Path", "Machine")
    $user = [Environment]::GetEnvironmentVariable("Path", "User")
    if ($machine -and $user) {
        $env:Path = "$machine;$user"
    } elseif ($machine) {
        $env:Path = $machine
    } elseif ($user) {
        $env:Path = $user
    }
}

function Prepend-GnuWin32 {
    # Use GetFolderPath so we never rely on ${env:ProgramFiles(x86)} tokenization; Join-Path handles spaces.
    $candidates = @()
    foreach ($root in @(
            [Environment]::GetFolderPath([Environment+SpecialFolder]::ProgramFilesX86),
            [Environment]::GetFolderPath([Environment+SpecialFolder]::ProgramFiles)
        )) {
        if ([string]::IsNullOrWhiteSpace($root)) { continue }
        $candidates += (Join-Path $root "GnuWin32\bin")
    }
    foreach ($d in $candidates) {
        if (Test-Path -LiteralPath (Join-Path $d "make.exe")) {
            $env:Path = "$d;$env:Path"
            Write-Host "install-make: prepended to PATH: $d"
            return $true
        }
    }
    return $false
}

if ($env:SKIP_AUTO_MAKE -eq "1") {
    Write-Error "install-make: SKIP_AUTO_MAKE=1 - install GNU Make manually."
    exit 1
}

Refresh-Path
if (Test-GnuMake) {
    make --version
    Write-Host "install-make: OK"
    exit 0
}

Write-Host "install-make: GNU Make not found - trying installers..."

$winget = Get-Command winget -ErrorAction SilentlyContinue
if ($winget) {
    try {
        & "$($winget.Source)" @(
            "install", "-e", "--id", "GnuWin32.Make",
            "--accept-package-agreements", "--accept-source-agreements", "--disable-interactivity"
        )
    } catch {
        Write-Host "install-make: winget install failed: $_"
    }
    Refresh-Path
    Prepend-GnuWin32 | Out-Null
    if (Test-GnuMake) {
        make --version
        Write-Host "install-make: OK"
        exit 0
    }
}

$choco = Get-Command choco -ErrorAction SilentlyContinue
if ($choco) {
    try {
        & "$($choco.Source)" install make -y
    } catch {
        Write-Host "install-make: choco install failed: $_"
    }
    Refresh-Path
    if (Test-GnuMake) {
        make --version
        Write-Host "install-make: OK"
        exit 0
    }
}

$scoop = Get-Command scoop -ErrorAction SilentlyContinue
if ($scoop) {
    try {
        & "$($scoop.Source)" install make
    } catch {
        Write-Host "install-make: scoop install failed: $_"
    }
    Refresh-Path
    if (Test-GnuMake) {
        make --version
        Write-Host "install-make: OK"
        exit 0
    }
}

Refresh-Path
Prepend-GnuWin32 | Out-Null
if (Test-GnuMake) {
    make --version
    Write-Host "install-make: OK"
    exit 0
}

Write-Error @"
install-make: could not install GNU Make automatically.
  Try (elevated if needed): winget install -e --id GnuWin32.Make
  Or: choco install make   Or: scoop install make
  From Git Bash you can run: bash scripts/install-make.sh
"@
exit 1
