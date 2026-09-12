# ============================================================
# Costra CLI installer — Windows
# Served from: https://get.costraai.com/windows
# Usage: irm https://get.costraai.com/windows | iex
# ============================================================

$ErrorActionPreference = "Stop"

$Repo = "shubmeshaws/costractl"
$InstallDir = "$env:LOCALAPPDATA\costra\bin"
$BinaryName = "costractl.exe"

Write-Host "Fetching latest release info..."
$LatestRelease = Invoke-RestMethod -Uri "https://api.github.com/repos/$Repo/releases/latest"
$LatestTag = $LatestRelease.tag_name

if (-not $LatestTag) {
    Write-Error "Could not determine latest version. Check https://github.com/$Repo/releases"
    exit 1
}

$Asset = "costractl-windows-amd64.exe"
$DownloadUrl = "https://github.com/$Repo/releases/download/$LatestTag/$Asset"
$ChecksumUrl = "https://github.com/$Repo/releases/download/$LatestTag/checksums.txt"

$TmpDir = New-Item -ItemType Directory -Path (Join-Path $env:TEMP ([System.Guid]::NewGuid()))
$BinaryPath = Join-Path $TmpDir $BinaryName
$ChecksumPath = Join-Path $TmpDir "checksums.txt"

Write-Host "Downloading $Asset ($LatestTag)..."
Invoke-WebRequest -Uri $DownloadUrl -OutFile $BinaryPath

Write-Host "Downloading checksums for verification..."
Invoke-WebRequest -Uri $ChecksumUrl -OutFile $ChecksumPath

Write-Host "Verifying checksum..."
$ChecksumLine = Get-Content $ChecksumPath | Where-Object { $_ -match [regex]::Escape($Asset) }
if ($ChecksumLine) {
    $ExpectedSha = ($ChecksumLine -split '\s+')[0]
    $ActualSha = (Get-FileHash -Path $BinaryPath -Algorithm SHA256).Hash.ToLower()
    if ($ExpectedSha -ne $ActualSha) {
        Write-Error "Checksum mismatch! Expected $ExpectedSha, got $ActualSha. Aborting install."
        Remove-Item -Recurse -Force $TmpDir
        exit 1
    }
    Write-Host "Checksum verified."
} else {
    Write-Warning "No checksum entry found for $Asset. Proceeding without verification."
}

New-Item -ItemType Directory -Force -Path $InstallDir | Out-Null
Move-Item -Force $BinaryPath (Join-Path $InstallDir $BinaryName)

$UserPath = [Environment]::GetEnvironmentVariable("Path", "User")
if ($UserPath -notlike "*$InstallDir*") {
    [Environment]::SetEnvironmentVariable("Path", "$UserPath;$InstallDir", "User")
    Write-Host "Added $InstallDir to your user PATH. Restart your terminal for it to take effect."
}

Remove-Item -Recurse -Force $TmpDir

Write-Host ""
Write-Host "costractl $LatestTag installed successfully."
Write-Host "Run 'costractl --version' to verify, or 'costractl cluster connect --help' to get started."
