# irm https://raw.githubusercontent.com/luynrs/justray/main/install.ps1 | iex
# re-run it to update

$ErrorActionPreference = "Stop"
$ProgressPreference = "SilentlyContinue"

$dir = if ($env:JUSTRAY_INSTALL_DIR) { $env:JUSTRAY_INSTALL_DIR } else { "$env:LOCALAPPDATA\justray" }
$base = "https://github.com/luynrs/justray/releases/latest/download"
$arch = if ($env:PROCESSOR_ARCHITECTURE -eq "ARM64") { "arm64" } else { "amd64" }

$tmp = New-Item -ItemType Directory -Path (Join-Path ([System.IO.Path]::GetTempPath()) ([System.Guid]::NewGuid()))
try {
	Write-Host "install.ps1: fetching latest release"
	$checksums = Join-Path $tmp "checksums.txt"
	Invoke-WebRequest "$base/checksums.txt" -OutFile $checksums -UseBasicParsing
	$line = Get-Content $checksums | Where-Object { $_ -match "justray_.*_windows_$arch\.zip$" } | Select-Object -First 1
	if (-not $line) { throw "no release archive for windows_$arch" }
	$hash, $archive = $line.Trim() -split '\s+'

	Write-Host "install.ps1: downloading $archive"
	$zip = Join-Path $tmp $archive
	Invoke-WebRequest "$base/$archive" -OutFile $zip -UseBasicParsing
	if ((Get-FileHash $zip -Algorithm SHA256).Hash -ne $hash) { throw "checksum mismatch for $archive" }

	Expand-Archive $zip -DestinationPath "$tmp\out" -Force
	Copy-Item "$tmp\out\justray.exe" "$tmp\out\jray.exe"

	New-Item -ItemType Directory -Force -Path $dir | Out-Null
	Get-ChildItem $dir -Filter *.old | Remove-Item -Force -ErrorAction SilentlyContinue
	foreach ($exe in "justray.exe", "justrayd.exe", "jray.exe") {
		$dst = Join-Path $dir $exe
		# a running exe cannot be written over, only renamed away
		if (Test-Path $dst) { Move-Item $dst "$dst.old" -Force }
		Move-Item "$tmp\out\$exe" $dst -Force
	}

	Write-Host "install.ps1: installed justray, jray, justrayd to $dir"
	$userPath = [Environment]::GetEnvironmentVariable("Path", "User")
	if (($userPath -split ';') -notcontains $dir) {
		[Environment]::SetEnvironmentVariable("Path", "$userPath;$dir".Trim(';'), "User")
		Write-Host "install.ps1: added $dir to your PATH, restart your terminal"
	}
	if (Get-Process justrayd -ErrorAction SilentlyContinue) {
		Write-Host "install.ps1: justrayd is still running the old build, restart it: Stop-Process -Name justrayd"
	}
} finally {
	Remove-Item -Recurse -Force $tmp
}
