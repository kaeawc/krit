#Requires -Version 5.1
<#
.SYNOPSIS
    Installs krit (Kotlin static analysis) on Windows.
.DESCRIPTION
    Interactive installer that supports Scoop, Go install, and direct binary download
    from GitHub Releases. Optionally installs krit-lsp and krit-mcp.
.PARAMETER Version
    Release version to install (default: latest).
.PARAMETER InstallDir
    Directory to place the binary when downloading from releases.
#>
[CmdletBinding()]
param(
    [string]$Version = $env:KRIT_VERSION ?? "latest",
    [string]$InstallDir = $env:KRIT_INSTALL_DIR ?? "$env:LOCALAPPDATA\krit\bin"
)

Set-StrictMode -Version Latest
$ErrorActionPreference = "Stop"

$Repo = "kaeawc/krit"

# ---------------------------------------------------------------------------
# Helpers
# ---------------------------------------------------------------------------
function Write-Info    { param([string]$Msg) Write-Host "[info] " -ForegroundColor Blue -NoNewline; Write-Host $Msg }
function Write-Ok      { param([string]$Msg) Write-Host "[ok]   " -ForegroundColor Green -NoNewline; Write-Host $Msg }
function Write-Warn    { param([string]$Msg) Write-Host "[warn] " -ForegroundColor Yellow -NoNewline; Write-Host $Msg }
function Write-Err     { param([string]$Msg) Write-Host "[error]" -ForegroundColor Red -NoNewline; Write-Host " $Msg"; exit 1 }

# ---------------------------------------------------------------------------
# Platform detection
# ---------------------------------------------------------------------------
function Get-Platform {
    $os = "windows"
    $arch = if ([System.Environment]::Is64BitOperatingSystem) {
        if ($env:PROCESSOR_ARCHITECTURE -eq "ARM64") { "arm64" } else { "amd64" }
    } else {
        Write-Err "32-bit Windows is not supported."
    }
    return "${os}_${arch}"
}

# ---------------------------------------------------------------------------
# Check for gum
# ---------------------------------------------------------------------------
$HasGum = $null -ne (Get-Command gum -ErrorAction SilentlyContinue)

function Read-Choice {
    param(
        [string]$Header,
        [string[]]$Options
    )

    if ($HasGum) {
        $result = & gum choose --header $Header @Options
        return $result
    }

    Write-Host ""
    Write-Host $Header -ForegroundColor Cyan
    for ($i = 0; $i -lt $Options.Count; $i++) {
        Write-Host "  $($i + 1)) $($Options[$i])"
    }
    $num = Read-Host "Enter choice [1-$($Options.Count)]"
    $idx = [int]$num - 1
    if ($idx -lt 0 -or $idx -ge $Options.Count) {
        Write-Err "Invalid selection."
    }
    return $Options[$idx]
}

# ---------------------------------------------------------------------------
# Choose installation method
# ---------------------------------------------------------------------------
function Choose-Method {
    $methods = @()

    if (Get-Command scoop -ErrorAction SilentlyContinue) {
        $methods += "Scoop  (scoop install krit)"
    }
    if (Get-Command go -ErrorAction SilentlyContinue) {
        $methods += "Go install  (go install github.com/$Repo/cmd/krit@latest)"
    }
    $methods += "Download binary from GitHub Releases"

    $choice = Read-Choice -Header "How would you like to install krit?" -Options $methods

    switch -Wildcard ($choice) {
        "Scoop*"      { return "scoop" }
        "Go install*" { return "go" }
        "Download*"   { return "binary" }
        default       { Write-Err "Unknown selection: $choice" }
    }
}

# ---------------------------------------------------------------------------
# Install methods
# ---------------------------------------------------------------------------
function Install-Scoop {
    Write-Info "Installing via Scoop..."
    scoop install krit
}

function Install-Go {
    Write-Info "Installing via go install..."
    if ($Version -eq "latest") {
        go install "github.com/$Repo/cmd/krit@latest"
    } else {
        $v = $Version -replace "^v", ""
        go install "github.com/$Repo/cmd/krit@v$v"
    }
}

function Install-Binary {
    Write-Info "Installing from GitHub Releases..."

    $platform = Get-Platform
    $tag = $Version
    if ($tag -eq "latest") {
        $release = Invoke-RestMethod -Uri "https://api.github.com/repos/$Repo/releases/latest"
        $tag = $release.tag_name
        if (-not $tag) { Write-Err "Could not determine latest release tag." }
        Write-Info "Latest release: $tag"
    }

    $ver = $tag -replace "^v", ""
    $archive = "krit_${ver}_${platform}.zip"
    $baseUrl = "https://github.com/$Repo/releases/download/$tag"
    $url = "$baseUrl/$archive"

    $tmpDir = Join-Path ([System.IO.Path]::GetTempPath()) "krit-install-$(Get-Random)"
    New-Item -ItemType Directory -Path $tmpDir -Force | Out-Null

    try {
        Write-Info "Downloading $archive..."
        Invoke-WebRequest -Uri $url -OutFile (Join-Path $tmpDir $archive) -UseBasicParsing

        # Checksum verification
        $checksumsUrl = "$baseUrl/checksums.txt"
        try {
            Invoke-WebRequest -Uri $checksumsUrl -OutFile (Join-Path $tmpDir "checksums.txt") -UseBasicParsing
            $checksumLine = Get-Content (Join-Path $tmpDir "checksums.txt") | Where-Object { $_ -match $archive }
            if ($checksumLine) {
                $expected = ($checksumLine -split "\s+")[0]
                $actual = (Get-FileHash -Path (Join-Path $tmpDir $archive) -Algorithm SHA256).Hash.ToLower()
                if ($expected -ne $actual) {
                    Write-Err "Checksum mismatch! Expected $expected, got $actual."
                }
                Write-Ok "Checksum verified."
            } else {
                Write-Warn "Archive not found in checksums.txt; skipping verification."
            }
        } catch {
            Write-Warn "checksums.txt not available; skipping verification."
        }

        # Extract
        if (-not (Test-Path $InstallDir)) {
            New-Item -ItemType Directory -Path $InstallDir -Force | Out-Null
        }
        Expand-Archive -Path (Join-Path $tmpDir $archive) -DestinationPath $tmpDir -Force
        Copy-Item -Path (Join-Path $tmpDir "krit.exe") -Destination (Join-Path $InstallDir "krit.exe") -Force
        Write-Ok "Installed krit to $InstallDir\krit.exe"

        # Add to PATH if not already present
        $currentPath = [Environment]::GetEnvironmentVariable("Path", "User")
        if ($currentPath -notlike "*$InstallDir*") {
            Write-Warn "$InstallDir is not on your PATH."
            $addPath = Read-Host "Add it to your user PATH? [Y/n]"
            if ($addPath -ne "n") {
                [Environment]::SetEnvironmentVariable("Path", "$currentPath;$InstallDir", "User")
                $env:Path = "$env:Path;$InstallDir"
                Write-Ok "Added $InstallDir to user PATH. Restart your shell for it to take effect."
            }
        }
    } finally {
        Remove-Item -Recurse -Force $tmpDir -ErrorAction SilentlyContinue
    }
}

# ---------------------------------------------------------------------------
# Verify
# ---------------------------------------------------------------------------
function Test-Installation {
    Write-Host ""
    $krit = Get-Command krit -ErrorAction SilentlyContinue
    if ($krit) {
        $ver = & krit --version 2>&1
        Write-Ok "krit is installed: $ver"
    } else {
        Write-Warn "krit not found on PATH."
        Write-Warn "You may need to add $InstallDir to your PATH or restart your shell."
    }
}

# ---------------------------------------------------------------------------
# Optional companion tools
# ---------------------------------------------------------------------------
function Install-Extras {
    Write-Host ""
    $options = @(
        "Yes, install krit-lsp and krit-mcp",
        "Just krit-lsp",
        "Just krit-mcp",
        "No thanks"
    )
    $answer = Read-Choice -Header "Install companion tools?" -Options $options

    $installLsp = $false
    $installMcp = $false
    switch ($answer) {
        "Yes, install krit-lsp and krit-mcp" { $installLsp = $true; $installMcp = $true }
        "Just krit-lsp" { $installLsp = $true }
        "Just krit-mcp" { $installMcp = $true }
        default { return }
    }

    if ($installLsp) {
        Write-Info "Installing krit-lsp..."
        if (Get-Command go -ErrorAction SilentlyContinue) {
            go install "github.com/$Repo/cmd/krit-lsp@latest"
            Write-Ok "krit-lsp installed."
        } else {
            Write-Warn "go not found; skipping krit-lsp (requires Go toolchain)."
        }
    }

    if ($installMcp) {
        Write-Info "Installing krit-mcp..."
        if (Get-Command go -ErrorAction SilentlyContinue) {
            go install "github.com/$Repo/cmd/krit-mcp@latest"
            Write-Ok "krit-mcp installed."
        } else {
            Write-Warn "go not found; skipping krit-mcp (requires Go toolchain)."
        }
    }
}

# ---------------------------------------------------------------------------
# Main
# ---------------------------------------------------------------------------
Write-Host ""
Write-Host "krit installer" -ForegroundColor Cyan
Write-Host "Repository: github.com/$Repo"
Write-Host ""

$platform = Get-Platform
Write-Info "Detected platform: $platform"

$method = Choose-Method

switch ($method) {
    "scoop"  { Install-Scoop }
    "go"     { Install-Go }
    "binary" { Install-Binary }
}

Test-Installation
Install-Extras

Write-Host ""
Write-Ok "All done! Run 'krit --help' to get started."
