[CmdletBinding()]
param(
    [string]$Owner = "doodooapp",
    [string]$Repository = "nivra-releases"
)
$ErrorActionPreference = "Stop"
$base = "https://$Owner.github.io/$Repository"
Write-Host "Kontrollerer $base ..." -ForegroundColor Cyan
$health = Invoke-RestMethod -Uri "$base/health.json" -Headers @{"Cache-Control"="no-cache"}
$status = Invoke-RestMethod -Uri "$base/status.json" -Headers @{"Cache-Control"="no-cache"}
$health | ConvertTo-Json -Depth 5
$status | ConvertTo-Json -Depth 8
