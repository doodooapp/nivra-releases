[CmdletBinding()]
param(
    [string]$Output = (Join-Path ([Environment]::GetFolderPath("MyDocuments")) "Nivra-Release-Signing-Key-Backup.nivrakey"),
    [string]$ReleaseDirectory = (Join-Path ([Environment]::GetFolderPath("MyDocuments")) "Nivra Release Server")
)
$ErrorActionPreference = "Stop"
$tool = Join-Path $ReleaseDirectory "tools\NivraRelease.exe"
if (-not (Test-Path -LiteralPath $tool -PathType Leaf)) { throw "Release-verktøyet finst ikkje. Køyr oppsettet først." }
function Plain([Security.SecureString]$Secure) {
    $ptr = [Runtime.InteropServices.Marshal]::SecureStringToBSTR($Secure)
    try { return [Runtime.InteropServices.Marshal]::PtrToStringBSTR($ptr) }
    finally { [Runtime.InteropServices.Marshal]::ZeroFreeBSTR($ptr) }
}
$one = Plain (Read-Host "Recovery-passord, minst 12 teikn" -AsSecureString)
$two = Plain (Read-Host "Gjenta recovery-passordet" -AsSecureString)
try {
    if ($one -cne $two) { throw "Passorda var ikkje like." }
    if ($one.Length -lt 12) { throw "Passordet må ha minst 12 teikn." }
    $env:NIVRA_RELEASE_BACKUP_PASSPHRASE = $one
    & $tool backup-key --out $Output
    if ($LASTEXITCODE -ne 0) { throw "Backup feila." }
    Write-Host "Backup lagra: $Output" -ForegroundColor Green
} finally {
    Remove-Item Env:NIVRA_RELEASE_BACKUP_PASSPHRASE -ErrorAction SilentlyContinue
    $one = $null; $two = $null
}
