# Run script for netFlow_tool
# Must be run as Administrator (WinDivert requires it)
#
# The UI now automatically launches and manages the Rust core process.
# You only need to run one executable.

$ErrorActionPreference = "Stop"

$buildDir = "$PSScriptRoot\..\build"

if (-not (Test-Path "$buildDir\netFlow_tool-ui.exe")) {
    Write-Host "Build not found. Run build.ps1 first." -ForegroundColor Red
    exit 1
}

Write-Host "=== Starting netFlow_tool ===" -ForegroundColor Cyan
Write-Host "UI will automatically launch the core engine." -ForegroundColor Yellow

# Run the UI — it handles core lifecycle automatically
& "$buildDir\netFlow_tool-ui.exe"

Write-Host "Done." -ForegroundColor Green
