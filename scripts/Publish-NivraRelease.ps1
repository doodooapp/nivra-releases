[CmdletBinding()]
param(
    [Parameter(Mandatory=$true)]
    [string]$Installer,

    [Parameter(Mandatory=$true)]
    [string]$Version,

    [ValidateSet("alpha", "beta", "stable")]
    [string]$Channel = "alpha",

    [string]$NotesFile,
    [string]$MinimumVersion = "",
    [switch]$Mandatory,
    [ValidateRange(0,100)]
    [int]$Rollout = 100,
    [string]$Owner = "doodooapp",
    [string]$Repository = "nivra-releases",
    [string]$ReleaseDirectory = (Join-Path ([Environment]::GetFolderPath("MyDocuments")) "Nivra Release Server")
)

$ErrorActionPreference = "Stop"
if (-not (Test-Path -LiteralPath $Installer -PathType Leaf)) { throw "Installasjonsfila finst ikkje: $Installer" }
if (-not (Test-Path -LiteralPath $ReleaseDirectory -PathType Container)) { throw "Release-mappa finst ikkje. Køyr SETUP-RELEASE-SERVER.cmd først." }

$tool = Join-Path $ReleaseDirectory "tools\NivraRelease.exe"
if (-not (Test-Path -LiteralPath $tool -PathType Leaf)) { throw "NivraRelease.exe manglar i release-mappa." }

$args = @(
    "publish",
    "--owner", $Owner,
    "--repo", $Repository,
    "--workdir", $ReleaseDirectory,
    "--installer", (Resolve-Path -LiteralPath $Installer).Path,
    "--version", $Version,
    "--channel", $Channel,
    "--minimum-version", $MinimumVersion,
    "--rollout", $Rollout
)
if ($Mandatory) { $args += "--mandatory" }
if ($NotesFile) {
    if (-not (Test-Path -LiteralPath $NotesFile -PathType Leaf)) { throw "Release notes-fila finst ikkje: $NotesFile" }
    $args += @("--notes-file", (Resolve-Path -LiteralPath $NotesFile).Path)
} else {
    $notes = Read-Host "Skriv ei kort release-melding"
    if ($notes) { $args += @("--notes", $notes) }
}

Write-Host "Publiserer Nivra $Version til $Channel..." -ForegroundColor Cyan
& $tool @args
if ($LASTEXITCODE -ne 0) { throw "Publiseringa feila med kode $LASTEXITCODE." }
Write-Host "Publisering fullført." -ForegroundColor Green
