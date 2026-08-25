# irm https://raw.githubusercontent.com/luynrs/justray/main/install.ps1 | iex
$ErrorActionPreference = "Stop"
$ProgressPreference = "SilentlyContinue"

$dir = if ($env:JUSTRAY_INSTALL_DIR) { $env:JUSTRAY_INSTALL_DIR } else { "$env:LOCALAPPDATA\justray" }
$base = "https://github.com/luynrs/justray/releases/latest/download"
$arch = if ($env:PROCESSOR_ARCHITECTURE -eq "ARM64") { "arm64" } else { "amd64" }

$tmp = New-Item -ItemType Directory -Path (Join-Path ([System.IO.Path]::GetTempPath()) ([System.Guid]::NewGuid()))
try {
	Write-Host "install.ps1: fetching latest release..."
	$line = (Invoke-WebRequest "$base/checksums.txt" -UseBasicParsing).Content -split "`n" |
		Where-Object { $_ -match "justray_.*_windows_$arch\.zip\s*$" } | Select-Object -First 1
	if (-not $line) { throw "no release archive for windows_$arch" }
	$archive = ($line.Trim() -split '\s+')[1]

	Write-Host "install.ps1: downloading $archive..."
	$zip = Join-Path $tmp $archive
	Invoke-WebRequest "$base/$archive" -OutFile $zip -UseBasicParsing
	if ((Get-FileHash $zip -Algorithm SHA256).Hash -ne ($line.Trim() -split '\s+')[0]) {
		throw "checksum mismatch for $archive"
	}

	New-Item -ItemType Directory -Force -Path $dir | Out-Null
	Expand-Archive $zip -DestinationPath $dir -Force
	Copy-Item "$dir\justray.exe" "$dir\jray.exe" -Force

	Write-Host "install.ps1: installed justray, jray, justrayd to $dir"

	$userPath = [Environment]::GetEnvironmentVariable("Path", "User")
	if (($userPath -split ';') -notcontains $dir) {
		[Environment]::SetEnvironmentVariable("Path", "$userPath;$dir".Trim(';'), "User")
		Write-Host "install.ps1: added $dir to your PATH, restart your terminal"
	}
} finally {
	Remove-Item -Recurse -Force $tmp
}
