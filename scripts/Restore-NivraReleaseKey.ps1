[CmdletBinding()]
param(
    [Parameter(Mandatory=$true)] [string]$Backup,
    [string]$ReleaseDirectory = (Join-Path ([Environment]::GetFolderPath("MyDocuments")) "Nivra Release Server")
)
$ErrorActionPreference = "Stop"
$tool = Join-Path $ReleaseDirectory "tools\NivraRelease.exe"
if (-not (Test-Path -LiteralPath $tool -PathType Leaf)) { throw "Release-verktøyet finst ikkje." }
if (-not (Test-Path -LiteralPath $Backup -PathType Leaf)) { throw "Backup-fila finst ikkje: $Backup" }
$secure = Read-Host "Recovery-passord" -AsSecureString
$ptr = [Runtime.InteropServices.Marshal]::SecureStringToBSTR($secure)
try { $plain = [Runtime.InteropServices.Marshal]::PtrToStringBSTR($ptr) }
finally { [Runtime.InteropServices.Marshal]::ZeroFreeBSTR($ptr) }
try {
    $env:NIVRA_RELEASE_BACKUP_PASSPHRASE = $plain
    & $tool restore-key --in (Resolve-Path -LiteralPath $Backup).Path --force
    if ($LASTEXITCODE -ne 0) { throw "Gjenoppretting feila." }
    Write-Host "Signeringsnøkkelen er gjenoppretta." -ForegroundColor Green
} finally {
    Remove-Item Env:NIVRA_RELEASE_BACKUP_PASSPHRASE -ErrorAction SilentlyContinue
    $plain = $null
}
