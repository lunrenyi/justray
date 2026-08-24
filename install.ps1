# irm https://raw.githubusercontent.com/luynrs/justray/main/install.ps1 | iex
$ErrorActionPreference = "Stop"

$repo = "luynrs/justray"
$dir = if ($env:JUSTRAY_INSTALL_DIR) { $env:JUSTRAY_INSTALL_DIR } else { "$env:LOCALAPPDATA\justray" }

$arch = if ($env:PROCESSOR_ARCHITECTURE -eq "ARM64") { "arm64" } else { "amd64" }

$tmp = New-Item -ItemType Directory -Path ([System.IO.Path]::GetTempPath() + [System.Guid]::NewGuid())
try {
	Write-Host "install.ps1: fetching latest release..."
	$release = Invoke-RestMethod "https://api.github.com/repos/$repo/releases/latest"
	$asset = $release.assets | Where-Object { $_.name -like "justray_*_windows_$arch.zip" } | Select-Object -First 1
	if (-not $asset) { throw "no release archive for windows_$arch" }

	$zip = "$tmp\$($asset.name)"
	Write-Host "install.ps1: downloading $($asset.name)..."
	Invoke-WebRequest $asset.browser_download_url -OutFile $zip

	New-Item -ItemType Directory -Force -Path $dir | Out-Null
	Expand-Archive $zip -DestinationPath $dir -Force
	Copy-Item "$dir\justray.exe" "$dir\jray.exe" -Force

	Write-Host "install.ps1: installed justray, jray, justrayd to $dir"

	$userPath = [Environment]::GetEnvironmentVariable("Path", "User")
	if ($userPath -notlike "*$dir*") {
		[Environment]::SetEnvironmentVariable("Path", "$userPath;$dir", "User")
		Write-Host "install.ps1: added $dir to your PATH, restart your terminal"
	}
} finally {
	Remove-Item -Recurse -Force $tmp
}
