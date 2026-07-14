# Redfetch

A tiny red terminal system fetch with a cleaner ASCII badge.

Install:

```powershell
# PowerShell
iwr -UseBasicParsing https://raw.githubusercontent.com/TechnoRed2026/redfetch/main/add_alias.ps1 -OutFile $env:TEMP\redfetch.ps1; powershell -ExecutionPolicy Bypass -File $env:TEMP\redfetch.ps1

# cmd
curl -L -o "%TEMP%\add_alias.bat" https://raw.githubusercontent.com/TechnoRed2026/redfetch/main/add_alias.bat && "%TEMP%\add_alias.bat"

# Linux/macOS
curl -fsSL https://raw.githubusercontent.com/TechnoRed2026/redfetch/main/add_alias.sh | sh
```

The installer clones/updates the repo, installs a portable Go toolchain if `go` is missing, builds Redfetch, adds it to PATH, then runs `redfetch`.

Run from source:

```powershell
go run .
```

Build:

```powershell
go build -o redfetch.exe .
.\redfetch.exe
```

Shows:

- user/host
- OS + arch
- CPU
- RAM
- all mounted disks
- LAN IP
- uptime

No external Go dependencies. One file, one binary.
