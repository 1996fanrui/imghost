#Requires -Version 5.1
<#
.SYNOPSIS
    One-line installer for filehub on Windows.

.DESCRIPTION
    Downloads the filehub CLI and filehubd daemon binaries from GitHub
    Releases into %LOCALAPPDATA%\Programs\filehub and adds that directory to
    the user PATH. No Windows service is registered; see the printed guidance
    for running filehubd under Task Scheduler.

.PARAMETER Version
    Explicit version tag to install, e.g. v0.1.0 or v0.1.0-alpha.1. When set,
    the -Pre switch is ignored.

.PARAMETER Pre
    Install the latest release including pre-releases (alpha channel).
    Without this switch or -Version, the installer selects the latest stable
    release.

.EXAMPLE
    iwr https://raw.githubusercontent.com/1996fanrui/filehub/main/install.ps1 -UseBasicParsing | iex
#>
[CmdletBinding()]
param(
    [string]$Version = "",
    [switch]$Pre
)

$ErrorActionPreference = 'Stop'

$Repo = '1996fanrui/filehub'
$InstallDir = Join-Path $env:LOCALAPPDATA 'Programs\filehub'

# Detect host architecture. PROCESSOR_ARCHITEW6432 is set when the current
# process runs under WoW64 (e.g. 32-bit PowerShell on 64-bit Windows, or x64
# PowerShell emulated on ARM64) and reports the *host* arch; otherwise fall
# back to PROCESSOR_ARCHITECTURE which reflects the process arch.
$rawArch = if ($env:PROCESSOR_ARCHITEW6432) { $env:PROCESSOR_ARCHITEW6432 } else { $env:PROCESSOR_ARCHITECTURE }
switch ($rawArch) {
    'AMD64' { $arch = 'amd64' }
    'ARM64' { $arch = 'arm64' }
    default { throw "Unsupported processor architecture: $rawArch" }
}

# Resolve the release tag via the GitHub REST API.
$headers = @{ 'User-Agent' = 'filehub-installer' }
if ($Version) {
    $tag = $Version
    Write-Host "Using explicit version: $tag"
} elseif ($Pre) {
    $url = "https://api.github.com/repos/$Repo/releases"
    $releases = Invoke-WebRequest -UseBasicParsing -Headers $headers -Uri $url | Select-Object -ExpandProperty Content | ConvertFrom-Json
    if (-not $releases -or $releases.Count -eq 0) {
        throw "No releases found for $Repo"
    }
    $tag = $releases[0].tag_name
    Write-Host "Resolved latest release (including pre-releases): $tag"
} else {
    $url = "https://api.github.com/repos/$Repo/releases/latest"
    $release = Invoke-WebRequest -UseBasicParsing -Headers $headers -Uri $url | Select-Object -ExpandProperty Content | ConvertFrom-Json
    $tag = $release.tag_name
    Write-Host "Resolved latest stable release: $tag"
}

# Ensure install directory exists.
if (-not (Test-Path -LiteralPath $InstallDir)) {
    New-Item -ItemType Directory -Path $InstallDir -Force | Out-Null
}

$cliAsset = "filehub_windows_${arch}.exe"
$daemonAsset = "filehubd_windows_${arch}.exe"
$cliUrl = "https://github.com/$Repo/releases/download/$tag/$cliAsset"
$daemonUrl = "https://github.com/$Repo/releases/download/$tag/$daemonAsset"

$cliPath = Join-Path $InstallDir 'filehub.exe'
$daemonPath = Join-Path $InstallDir 'filehubd.exe'

Write-Host "Downloading $cliAsset"
Invoke-WebRequest -UseBasicParsing -Headers $headers -Uri $cliUrl -OutFile $cliPath

Write-Host "Downloading $daemonAsset"
Invoke-WebRequest -UseBasicParsing -Headers $headers -Uri $daemonUrl -OutFile $daemonPath

# Append install dir to user PATH if not already present. The 'User' scope
# only rewrites HKCU, so no elevation is required.
$existingPath = [Environment]::GetEnvironmentVariable('Path', 'User')
if (-not $existingPath) { $existingPath = '' }
$pathEntries = $existingPath -split ';' | Where-Object { $_ -ne '' }
if ($pathEntries -notcontains $InstallDir) {
    $newPath = if ($existingPath) { "$existingPath;$InstallDir" } else { $InstallDir }
    [Environment]::SetEnvironmentVariable('Path', $newPath, 'User')
    Write-Host "Added $InstallDir to user PATH"
} else {
    Write-Host "$InstallDir is already on user PATH"
}

# Strip the leading 'v' from the tag for the summary line, matching the
# Unix installer's output style.
$displayVersion = if ($tag.StartsWith('v')) { $tag.Substring(1) } else { $tag }

Write-Host ""
Write-Host "Installation complete. filehub $displayVersion is ready."
Write-Host "  CLI    : $cliPath"
Write-Host "  daemon : $daemonPath"
Write-Host ""
Write-Host "PATH updated (user scope). Open a new PowerShell session for it to take effect."
Write-Host ""
Write-Host "Next steps:"
Write-Host "  - Run 'filehubd.exe' directly, OR"
Write-Host "  - Configure Windows Task Scheduler for background execution."
Write-Host "  - Swagger UI: http://localhost:34286/swagger/index.html"
