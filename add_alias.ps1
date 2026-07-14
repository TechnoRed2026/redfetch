param(
    [string]$RepoUrl = "https://github.com/TechnoRed2026/redfetch.git",
    [string]$InstallRoot = (Join-Path $env:LOCALAPPDATA "redfetch"),
    [string]$BinDir = (Join-Path $env:USERPROFILE ".local\bin")
)

Set-StrictMode -Version Latest
$ErrorActionPreference = "Stop"
[Net.ServicePointManager]::SecurityProtocol = [Net.SecurityProtocolType]::Tls12

function Say([string]$Text) { Write-Host "redfetch -> $Text" -ForegroundColor Red }
function Need([string]$Name) { Get-Command $Name -ErrorAction SilentlyContinue }

function Download([string]$Url, [string]$OutFile) {
    Say "download $Url"
    Invoke-WebRequest -UseBasicParsing -Uri $Url -OutFile $OutFile
}

function Add-UserPath([string]$Dir) {
    $Dir = [IO.Path]::GetFullPath($Dir)
    New-Item -ItemType Directory -Force -Path $Dir | Out-Null

    $userPath = [Environment]::GetEnvironmentVariable("Path", "User")
    $parts = @($userPath -split ";" | Where-Object { $_ })
    if ($parts -notcontains $Dir) {
        [Environment]::SetEnvironmentVariable("Path", ((@($parts) + $Dir) -join ";"), "User")
        Say "added to user PATH: $Dir"
    }
    if (($env:Path -split ";") -notcontains $Dir) {
        $env:Path = "$Dir;$env:Path"
    }
}

function Sync-Repo([string]$SrcDir) {
    New-Item -ItemType Directory -Force -Path $InstallRoot | Out-Null
    $git = Need "git"
    if ($git) {
        if (Test-Path (Join-Path $SrcDir ".git")) {
            Say "update repo"
            & $git.Source -C $SrcDir pull --ff-only
        } else {
            Remove-Item -Recurse -Force $SrcDir -ErrorAction SilentlyContinue
            Say "clone repo"
            & $git.Source clone --depth 1 $RepoUrl $SrcDir
        }
        return
    }

    Say "git not found; using GitHub zip"
    $zip = Join-Path $env:TEMP "redfetch-main.zip"
    $unpack = Join-Path $InstallRoot "zip"
    Download "https://github.com/TechnoRed2026/redfetch/archive/refs/heads/main.zip" $zip
    Remove-Item -Recurse -Force $unpack, $SrcDir -ErrorAction SilentlyContinue
    New-Item -ItemType Directory -Force -Path $unpack | Out-Null
    Expand-Archive -Path $zip -DestinationPath $unpack -Force
    Move-Item (Join-Path $unpack "redfetch-main") $SrcDir
}

function Ensure-Go {
    $cmd = Need "go"
    if ($cmd) { return $cmd.Source }

    Say "Go not found; downloading portable Go"
    $arch = switch ($env:PROCESSOR_ARCHITECTURE) { "ARM64" { "arm64" } default { "amd64" } }
    $version = ((Invoke-WebRequest -UseBasicParsing -Uri "https://go.dev/VERSION?m=text").Content -split "`n")[0].Trim()
    if (-not $version.StartsWith("go")) { throw "cannot detect latest Go version" }

    $file = "$version.windows-$arch.zip"
    $zip = Join-Path $env:TEMP $file
    $root = Join-Path $InstallRoot "go-portable"
    Download "https://go.dev/dl/$file" $zip
    Remove-Item -Recurse -Force $root -ErrorAction SilentlyContinue
    New-Item -ItemType Directory -Force -Path $root | Out-Null
    Expand-Archive -Path $zip -DestinationPath $root -Force

    $go = Join-Path $root "go\bin\go.exe"
    if (-not (Test-Path $go)) { throw "Go install failed: $go" }
    $env:Path = "$(Split-Path $go);$env:Path"
    return $go
}

$srcDir = Join-Path $InstallRoot "src"
Sync-Repo $srcDir
$go = Ensure-Go
Add-UserPath $BinDir
$out = Join-Path $BinDir "redfetch.exe"

Say "build"
Push-Location $srcDir
try {
    & $go build -trimpath "-ldflags=-s -w" -o $out .
} finally {
    Pop-Location
}

Say "installed: $out"
Write-Host ""
& $out
Write-Host ""
Write-Host "Open a new terminal, then run: redfetch" -ForegroundColor Green
