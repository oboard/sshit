# Installs the latest sshit Windows release for the current user.
# Usage: irm https://sshit.oboard.fun/install.ps1 | iex

$ErrorActionPreference = 'Stop'

$repo = 'oboard/sshit'

switch ([System.Runtime.InteropServices.RuntimeInformation]::OSArchitecture) {
  'X64' { $asset = 'sshit-windows-x64.exe' }
  'Arm64' { $asset = 'sshit-windows-arm64.exe' }
  default { throw "Unsupported architecture: $([System.Runtime.InteropServices.RuntimeInformation]::OSArchitecture)" }
}

$installDir = if ($env:INSTALL_DIR) { $env:INSTALL_DIR } else { Join-Path $env:LOCALAPPDATA 'sshit\bin' }
$installPath = Join-Path $installDir 'sshit.exe'
$releaseUrl = "https://github.com/$repo/releases/latest/download/$asset"
$mirrorUrl = "https://ghfast.top/$releaseUrl"
$tempPath = Join-Path ([System.IO.Path]::GetTempPath()) "sshit-$([System.Guid]::NewGuid().ToString('N')).exe"

try {
  Write-Host "Downloading $asset..."
  try {
    Invoke-WebRequest -Uri $releaseUrl -OutFile $tempPath
  }
  catch {
    Write-Warning 'GitHub download failed; retrying through the mirror.'
    Invoke-WebRequest -Uri $mirrorUrl -OutFile $tempPath
  }

  New-Item -ItemType Directory -Path $installDir -Force | Out-Null
  Move-Item -Path $tempPath -Destination $installPath -Force
}
finally {
  if (Test-Path $tempPath) {
    Remove-Item -Path $tempPath -Force
  }
}

$userPath = [Environment]::GetEnvironmentVariable('Path', 'User')
$userPathEntries = $userPath -split ';' | Where-Object { $_ }
if ($userPathEntries -notcontains $installDir) {
  $updatedUserPath = (@($userPathEntries) + $installDir) -join ';'
  [Environment]::SetEnvironmentVariable('Path', $updatedUserPath, 'User')
  $env:Path = "$installDir;$env:Path"
  Write-Host "Added $installDir to your user PATH. Open a new PowerShell window to use it."
}

Write-Host "Installed sshit to $installPath"
