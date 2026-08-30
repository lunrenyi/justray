#Requires -Version 5.1

$ErrorActionPreference = "Stop"
$ProgressPreference = "SilentlyContinue"

$repo = "https://github.com/luynrs/justray"
$version = if ($env:JUSTRAY_VERSION) { $env:JUSTRAY_VERSION } else { "latest" }
$dir = if ($env:JUSTRAY_INSTALL_DIR) {
	$env:JUSTRAY_INSTALL_DIR
} else {
	Join-Path $env:LOCALAPPDATA "justray"
}

$isTty = [Environment]::UserInteractive -and -not [Console]::IsOutputRedirected

function clear_line() {
	if ($isTty) {
		try {
			[Console]::SetCursorPosition(0, [Console]::CursorTop)
			Write-Host (" " * [Console]::BufferWidth) -NoNewline
			[Console]::SetCursorPosition(0, [Console]::CursorTop)
		} catch {}
	}
}

function step($msg) {
	if ($isTty) {
		clear_line
		Write-Host "• $msg" -NoNewline
	} else {
		Write-Host "• $msg"
	}
}

function done($msg) {
	clear_line
	Write-Host "✓ $msg"
}

function fail($msg) {
	clear_line
	if ($msg.Length -gt 0) {
		$msg = $msg.Substring(0, 1).ToUpper() + $msg.Substring(1)
	}
	[Console]::Error.WriteLine("✗ $msg")
	exit 1
}

$nativeArch = if ($env:PROCESSOR_ARCHITEW6432) {
	$env:PROCESSOR_ARCHITEW6432
} else {
	$env:PROCESSOR_ARCHITECTURE
}

$arch = switch ($nativeArch) {
	"AMD64" { "amd64" }
	"ARM64" { "arm64" }
	default { fail "unsupported arch: $nativeArch" }
}

$base = if ($version -eq "latest") {
	"$repo/releases/latest/download"
} else {
	"$repo/releases/download/$version"
}

if ($PSVersionTable.PSVersion.Major -lt 6) {
	[Net.ServicePointManager]::SecurityProtocol = [Net.SecurityProtocolType]::Tls12
}

function download($uri, $out) {
	for ($i = 1; $i -le 3; $i++) {
		try {
			Invoke-WebRequest -Uri $uri -OutFile $out -UseBasicParsing
			return
		} catch {
			if ($i -eq 3) { throw $_ }
			Start-Sleep -Seconds (1 * $i)
		}
	}
}

$tmp = Join-Path ([IO.Path]::GetTempPath()) ("justray-" + [guid]::NewGuid())
$restart = $false

New-Item -ItemType Directory -Path $tmp | Out-Null

try {
	step "Fetching release..."

	$checksums = Join-Path $tmp "checksums.txt"
	try {
		download "$base/checksums.txt" $checksums
	} catch {
		fail "failed to fetch release metadata"
	}

	$lines = @(
		Get-Content $checksums |
			Where-Object {
				$_ -match "^[0-9A-Fa-f]{64}\s+\*?justray_.*_windows_$arch\.zip$"
			}
	)

	if ($lines.Count -ne 1) {
		fail "expected exactly one release for windows_$arch"
	}

	$hash, $archive = $lines[0].Trim() -split '\s+', 2
	$archive = $archive.TrimStart("*")

	$tag = if ($archive -match "^justray_(.+)_[^_]+_[^_]+\.zip$") { $Matches[1] } else { $version }
	done "Found v$tag for windows/$arch"

	step "Downloading $archive..."

	$zip = Join-Path $tmp $archive
	try {
		download "$base/$archive" $zip
	} catch {
		fail "failed to download $archive"
	}

	if ((Get-FileHash $zip -Algorithm SHA256).Hash.ToLowerInvariant() -ne $hash.ToLowerInvariant()) {
		fail "checksum mismatch"
	}

	done "Verified checksum"

	$out = Join-Path $tmp "out"
	try {
		Expand-Archive $zip -DestinationPath $out -Force
	} catch {
		fail "failed to extract archive"
	}

	foreach ($exe in "justray.exe", "justrayd.exe") {
		if (-not (Test-Path (Join-Path $out $exe) -PathType Leaf)) {
			fail "archive is missing $exe"
		}
	}

	step "Installing..."

	$daemonExe = Join-Path $dir "justrayd.exe"
	$daemons = @(Get-Process justrayd -ErrorAction SilentlyContinue | Where-Object {
		try { $_.Path -eq $daemonExe } catch { $true }
	})

	if ($daemons.Count -gt 0) {
		$restart = $true
		if (Test-Path "$dir\justray.exe") {
			& "$dir\justray.exe" down *>$null
		} elseif (Get-Command jray -ErrorAction SilentlyContinue) {
			& (Get-Command jray).Source down *>$null
		}

		$daemons | Stop-Process -ErrorAction SilentlyContinue
		$sw = [Diagnostics.Stopwatch]::StartNew()
		while ($sw.ElapsedMilliseconds -lt 3000) {
			$remaining = @(Get-Process justrayd -ErrorAction SilentlyContinue | Where-Object {
				try { $_.Path -eq $daemonExe } catch { $true }
			})
			if ($remaining.Count -eq 0) { break }
			Start-Sleep -Milliseconds 100
		}

		$remaining = @(Get-Process justrayd -ErrorAction SilentlyContinue | Where-Object {
			try { $_.Path -eq $daemonExe } catch { $true }
		})
		if ($remaining.Count -gt 0) {
			$remaining | Stop-Process -Force -ErrorAction SilentlyContinue
		}
	}

	try {
		New-Item -ItemType Directory -Force -Path $dir | Out-Null
		Copy-Item "$out\justrayd.exe" "$dir\justrayd.exe" -Force
		Copy-Item "$out\justray.exe" "$dir\justray.exe" -Force
	} catch {
		fail "failed to install binaries"
	}

	$jray = Join-Path $dir "jray.exe"
	if (Test-Path $jray) {
		Remove-Item -Force $jray -ErrorAction SilentlyContinue
	}

	try {
		New-Item -ItemType HardLink -Path $jray -Target (Join-Path $dir "justray.exe") | Out-Null
	} catch {
		try {
			New-Item -ItemType SymbolicLink -Path $jray -Target "justray.exe" | Out-Null
		} catch {
			try {
				Copy-Item "$out\justray.exe" $jray -Force
			} catch {
				fail "failed to link jray"
			}
		}
	}

	$userPath = [Environment]::GetEnvironmentVariable("Path", "User")
	if ($null -eq $userPath) {
		$userPath = ""
	}

	if (($userPath -split ";") -notcontains $dir) {
		$userPath = "$($userPath.TrimEnd(";"));$dir".TrimStart(";")
		[Environment]::SetEnvironmentVariable("Path", $userPath, "User")
	}

	if (($env:Path -split ";") -notcontains $dir) {
		$env:Path = "$($env:Path.TrimEnd(";"));$dir"
	}

	done "Installed to $dir"
	Write-Host "`nTo get started, run jray in a new terminal window"
}
finally {
	Remove-Item -Recurse -Force $tmp -ErrorAction SilentlyContinue

	if ($restart -and -not (Get-Process justrayd -ErrorAction SilentlyContinue | Where-Object { try { $_.Path -eq (Join-Path $dir "justrayd.exe") } catch { $true } }) -and (Test-Path "$dir\justrayd.exe")) {
		Start-Process "$dir\justrayd.exe" -WindowStyle Hidden
	}
}
