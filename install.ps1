#Requires -Version 5.1
$ErrorActionPreference = 'Stop'
$ProgressPreference = 'SilentlyContinue'

$arch = if ($env:PROCESSOR_ARCHITECTURE -eq 'ARM64') { 'arm64' } else { 'amd64' }
$dest = "$env:LOCALAPPDATA\Programs\cc-statusline"
$exe = "$dest\cc-statusline.exe"

Write-Host "Fetching latest release..." -ForegroundColor Cyan
$release = Invoke-RestMethod 'https://api.github.com/repos/rileychh/cc-statusline/releases/latest'
$asset = $release.assets | Where-Object { $_.name -like "*windows_$arch.exe" } | Select-Object -First 1
if (-not $asset) {
    throw "No Windows $arch asset found in release $($release.tag_name)."
}

Write-Host "Downloading $($asset.name) -> $exe" -ForegroundColor Cyan
New-Item -ItemType Directory -Force -Path $dest | Out-Null
Invoke-WebRequest $asset.browser_download_url -OutFile $exe
Unblock-File $exe

$userPath = [Environment]::GetEnvironmentVariable('Path', 'User')
if ($userPath -notlike "*$dest*") {
    Write-Host "Adding $dest to user PATH" -ForegroundColor Cyan
    [Environment]::SetEnvironmentVariable('Path', "$userPath;$dest", 'User')
    $env:Path += ";$dest"
}

Write-Host "Installed cc-statusline $($release.tag_name)." -ForegroundColor Green
Write-Host "Restart any open shells so PATH takes effect, then add cc-statusline to ~/.claude/settings.json:" -ForegroundColor Yellow
Write-Host '  { "statusLine": { "type": "command", "command": "cc-statusline" } }'
