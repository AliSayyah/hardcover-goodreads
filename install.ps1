param(
    [string]$Repo = $env:REPO,
    [string]$Version = $env:VERSION,
    [string]$Bin = $env:BIN,
    [string]$BinDir = $env:BIN_DIR
)

$ErrorActionPreference = "Stop"
$ProgressPreference = "SilentlyContinue"

if (-not $Repo) { $Repo = "AliSayyah/hardcover-goodreads" }
if (-not $Version) { $Version = "latest" }
if (-not $Bin) { $Bin = "hardcover-goodreads" }
if (-not $BinDir) { $BinDir = Join-Path $env:LOCALAPPDATA "Programs\hardcover-goodreads\bin" }

if ($env:OS -ne "Windows_NT") {
    throw "install.ps1 is for Windows. Use install.sh on macOS or Linux."
}

try {
    $archName = [System.Runtime.InteropServices.RuntimeInformation]::OSArchitecture.ToString().ToLowerInvariant()
} catch {
    $archCandidates = @($env:PROCESSOR_ARCHITEW6432, $env:PROCESSOR_ARCHITECTURE) | Where-Object { $_ }
    $archName = $archCandidates[0].ToLowerInvariant()
}

switch -Regex ($archName) {
    "arm64|aarch64" { $Arch = "arm64"; break }
    "x64|amd64" { $Arch = "amd64"; break }
    default { throw "unsupported architecture: $archName" }
}

$Archive = "${Bin}_windows_${Arch}.zip"
$Base = "https://github.com/${Repo}/releases"
if ($Version -eq "latest") {
    $Url = "${Base}/latest/download/${Archive}"
} else {
    $Url = "${Base}/download/${Version}/${Archive}"
}

$Temp = Join-Path ([IO.Path]::GetTempPath()) ([IO.Path]::GetRandomFileName())
New-Item -ItemType Directory -Force -Path $Temp | Out-Null

try {
    $Zip = Join-Path $Temp $Archive
    $Exe = Join-Path $Temp "${Bin}.exe"
    $Target = Join-Path $BinDir "${Bin}.exe"

    New-Item -ItemType Directory -Force -Path $BinDir | Out-Null
    [Net.ServicePointManager]::SecurityProtocol = [Net.SecurityProtocolType]::Tls12
    $WebClient = New-Object Net.WebClient
    try {
        $WebClient.DownloadFile($Url, $Zip)
    } finally {
        $WebClient.Dispose()
    }
    Expand-Archive -Path $Zip -DestinationPath $Temp -Force
    Copy-Item -Force $Exe $Target

    $UserPath = [Environment]::GetEnvironmentVariable("Path", "User")
    $PathParts = @($UserPath -split ";" | Where-Object { $_ })
    if ($PathParts -notcontains $BinDir) {
        $NewPath = (@($BinDir) + $PathParts) -join ";"
        [Environment]::SetEnvironmentVariable("Path", $NewPath, "User")
        $env:Path = "${BinDir};$env:Path"
        Write-Host "added $BinDir to your user PATH"
    }

    Write-Host "installed $Bin to $Target"
    Write-Host "run: $Bin"
} finally {
    Remove-Item -Recurse -Force $Temp -ErrorAction SilentlyContinue
}
