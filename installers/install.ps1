<#
.SYNOPSIS
    Envapor installer for Windows.

.DESCRIPTION
    Downloads the matching Envapor release, verifies its checksum, installs
    envapor.exe under %LOCALAPPDATA%\Envapor\bin, and adds that directory to the
    user PATH.

.EXAMPLE
    irm https://raw.githubusercontent.com/automazeio/envapor/main/installers/install.ps1 | iex

.NOTES
    Environment overrides:
      ENVAPOR_VERSION  Release tag to install (e.g. v1.2.3). Defaults to latest.
      ENVAPOR_REPO     GitHub owner/repo to download from. Defaults to
                       automazeio/envapor (override for testing forks).
#>
#Requires -Version 5.1
$ErrorActionPreference = 'Stop'

$Repo = if ($env:ENVAPOR_REPO) { $env:ENVAPOR_REPO } else { 'automazeio/envapor' }
$Binary = 'envapor'

function Info($message) { Write-Host "envapor: $message" }
function Fail($message) { Write-Error "envapor: $message"; exit 1 }

$osArch = [System.Runtime.InteropServices.RuntimeInformation]::OSArchitecture
switch ($osArch) {
    'X64'   { $arch = 'amd64' }
    'Arm64' { $arch = 'arm64' }
    default { Fail "unsupported architecture: $osArch" }
}

$version = $env:ENVAPOR_VERSION
if (-not $version) {
    Info 'resolving latest release'
    $release = Invoke-RestMethod -Uri "https://api.github.com/repos/$Repo/releases/latest" `
        -Headers @{ 'User-Agent' = 'envapor-installer' }
    $version = $release.tag_name
}
if (-not $version) { Fail 'could not determine release version' }
$number = $version.TrimStart('v')

$asset = "${Binary}_${number}_windows_${arch}.zip"
$base = "https://github.com/$Repo/releases/download/$version"

$tmp = Join-Path $env:TEMP ('envapor-' + [System.Guid]::NewGuid().ToString('N'))
New-Item -ItemType Directory -Path $tmp | Out-Null
try {
    $zipPath = Join-Path $tmp $asset
    $sumsPath = Join-Path $tmp 'checksums.txt'

    Info "downloading $asset"
    Invoke-WebRequest -Uri "$base/$asset" -OutFile $zipPath -UseBasicParsing
    Invoke-WebRequest -Uri "$base/checksums.txt" -OutFile $sumsPath -UseBasicParsing

    Info 'verifying checksum'
    $expected = Get-Content $sumsPath | ForEach-Object {
        $parts = $_ -split '\s+'
        if ($parts.Length -ge 2 -and $parts[1] -eq $asset) { $parts[0] }
    } | Select-Object -First 1
    if (-not $expected) { Fail "checksum for $asset not found in checksums.txt" }
    $actual = (Get-FileHash -Path $zipPath -Algorithm SHA256).Hash.ToLower()
    if ($actual -ne $expected.ToLower()) { Fail "checksum mismatch for $asset" }

    Expand-Archive -Path $zipPath -DestinationPath $tmp -Force
    $exe = Join-Path $tmp "$Binary.exe"
    if (-not (Test-Path $exe)) { Fail "binary $Binary.exe not found in archive" }

    $installDir = Join-Path $env:LOCALAPPDATA 'Envapor\bin'
    New-Item -ItemType Directory -Path $installDir -Force | Out-Null
    Copy-Item -Path $exe -Destination (Join-Path $installDir "$Binary.exe") -Force
    Info "installed $Binary $version to $installDir\$Binary.exe"

    $userPath = [Environment]::GetEnvironmentVariable('Path', 'User')
    if (-not $userPath) { $userPath = '' }
    $entries = $userPath -split ';' | Where-Object { $_ -ne '' }
    if ($entries -notcontains $installDir) {
        $newPath = if ($userPath -eq '') { $installDir } else { "$userPath;$installDir" }
        [Environment]::SetEnvironmentVariable('Path', $newPath, 'User')
        Info "added $installDir to your user PATH"
        Info 'open a new terminal for the PATH change to take effect'
    }
} finally {
    Remove-Item -Recurse -Force $tmp -ErrorAction SilentlyContinue
}
