[CmdletBinding()]
param(
    [string]$Owner = "doodooapp",
    [string]$Repository = "nivra-releases",
    [string]$InstallDirectory = (Join-Path ([Environment]::GetFolderPath("MyDocuments")) "Nivra Release Server")
)

$ErrorActionPreference = "Stop"
$ProgressPreference = "SilentlyContinue"
$packageRoot = Split-Path -Parent $PSScriptRoot

function Write-Step([string]$Text) {
    Write-Host "`n==> $Text" -ForegroundColor Cyan
}

function Refresh-Path {
    $machine = [Environment]::GetEnvironmentVariable("Path", "Machine")
    $user = [Environment]::GetEnvironmentVariable("Path", "User")
    $env:Path = "$machine;$user"
}

# Windows PowerShell 5.1 can turn stderr from native programs into a terminating
# NativeCommandError when ErrorActionPreference is Stop. Commands used as probes
# (for example checking whether a repository exists) must therefore run with a
# temporary Continue policy and have their exit code inspected explicitly.
function Invoke-NativeProbe {
    param(
        [Parameter(Mandatory=$true)][string]$FilePath,
        [string[]]$Arguments = @()
    )

    $previousPreference = $ErrorActionPreference
    $output = @()
    $exitCode = 1
    try {
        $ErrorActionPreference = "Continue"
        $output = @(& $FilePath @Arguments 2>&1)
        $exitCode = $LASTEXITCODE
        if ($null -eq $exitCode) { $exitCode = 0 }
    } catch {
        $output = @($_)
        $exitCode = 1
    } finally {
        $ErrorActionPreference = $previousPreference
    }

    $text = (($output | ForEach-Object { $_.ToString() }) -join [Environment]::NewLine).Trim()
    return [pscustomobject]@{
        ExitCode = [int]$exitCode
        Output = $text
    }
}

function Invoke-NativeRequired {
    param(
        [Parameter(Mandatory=$true)][string]$FilePath,
        [string[]]$Arguments = @(),
        [Parameter(Mandatory=$true)][string]$FailureMessage
    )

    $previousPreference = $ErrorActionPreference
    try {
        $ErrorActionPreference = "Continue"
        & $FilePath @Arguments
        $exitCode = $LASTEXITCODE
        if ($null -eq $exitCode) { $exitCode = 0 }
    } finally {
        $ErrorActionPreference = $previousPreference
    }

    if ($exitCode -ne 0) {
        throw "$FailureMessage (exit $exitCode)."
    }
}

function Ensure-Tool([string]$Command, [string]$WingetId, [string]$DisplayName) {
    if (Get-Command $Command -ErrorAction SilentlyContinue) { return }
    if (-not (Get-Command winget.exe -ErrorAction SilentlyContinue)) {
        throw "$DisplayName manglar, og WinGet er ikkje tilgjengeleg. Installer $DisplayName manuelt og køyr oppsettet igjen."
    }
    Write-Step "Installerer $DisplayName"
    Invoke-NativeRequired -FilePath "winget.exe" -Arguments @(
        "install", "--id", $WingetId, "-e", "--source", "winget",
        "--accept-package-agreements", "--accept-source-agreements"
    ) -FailureMessage "Installering av $DisplayName feila"
    Refresh-Path
    if (-not (Get-Command $Command -ErrorAction SilentlyContinue)) {
        throw "$DisplayName blei installert, men $Command er framleis ikkje i PATH. Lukk dette vindauget, start PC-en på nytt og køyr oppsettet igjen."
    }
}

function Get-GitHubProfile {
    $probe = Invoke-NativeProbe -FilePath "gh.exe" -Arguments @("api", "user")
    if ($probe.ExitCode -ne 0) { return $null }
    try {
        return ($probe.Output | ConvertFrom-Json)
    } catch {
        throw "GitHub svarte, men brukarprofilen kunne ikkje lesast. Svar: $($probe.Output)"
    }
}

function Test-GitHubRepository {
    param([Parameter(Mandatory=$true)][string]$NameWithOwner)

    $probe = Invoke-NativeProbe -FilePath "gh.exe" -Arguments @("api", "repos/$NameWithOwner", "--silent")
    if ($probe.ExitCode -eq 0) {
        return [pscustomobject]@{ Exists = $true; Probe = $probe }
    }

    if ($probe.Output -match '(?i)(HTTP\s+404|not found|could not resolve to a repository)') {
        return [pscustomobject]@{ Exists = $false; Probe = $probe }
    }

    throw "Klarte ikkje å kontrollere repositoryet '$NameWithOwner'.`n$($probe.Output)"
}

Write-Host @"

NIVRA RELEASE SERVER 1.0.1
--------------------------
Dette oppsettet opprettar eller fullfører eit OFFENTLEG GitHub-repository:
  https://github.com/$Owner/$Repository

Repositoryet inneheld berre offentlege installasjonsfiler, signerte update-manifest
og release-verktøy. Nivra-kjeldekoden blir ikkje lasta opp.

Oppsettet kan trygt køyrast på nytt etter eit avbrot. Eksisterande .git-mappe,
signeringsnøkkel og release-data blir bevarte.
"@ -ForegroundColor White

$confirmation = Read-Host "Skriv PUBLIC for å stadfeste at release-filene kan vere offentlege"
if ($confirmation -cne "PUBLIC") {
    throw "Avbroten. Repositoryet blei ikkje oppretta."
}

Ensure-Tool "git.exe" "Git.Git" "Git"
Ensure-Tool "gh.exe" "GitHub.cli" "GitHub CLI"

Write-Step "Kontrollerer GitHub-innlogging"
$profile = Get-GitHubProfile
if ($null -eq $profile) {
    Write-Host "Nettlesaren blir opna for sikker GitHub-innlogging." -ForegroundColor Yellow
    Invoke-NativeRequired -FilePath "gh.exe" -Arguments @(
        "auth", "login", "--hostname", "github.com", "--git-protocol", "https", "--web"
    ) -FailureMessage "GitHub-innlogginga blei ikkje fullført"
    $profile = Get-GitHubProfile
}
if ($null -eq $profile -or -not $profile.login) {
    throw "Klarte ikkje å lese GitHub-brukaren etter innlogging."
}

Invoke-NativeRequired -FilePath "gh.exe" -Arguments @("auth", "setup-git") -FailureMessage "Klarte ikkje å konfigurere Git-autentisering gjennom GitHub CLI"

$login = [string]$profile.login
$userId = [string]$profile.id
if ($login -ne $Owner) {
    Write-Warning "Du er logga inn som '$login', medan oppsettet er konfigurert for '$Owner'."
    $useLogin = Read-Host "Skriv USE $login for å bruke den innlogga kontoen"
    if ($useLogin -cne "USE $login") { throw "Avbroten på grunn av ulik GitHub-konto." }
    $Owner = $login
}

$repoFull = "$Owner/$Repository"
$expectedRemote = "https://github.com/$repoFull.git"

Write-Step "Kontrollerer GitHub-repositoryet"
$repoState = Test-GitHubRepository -NameWithOwner $repoFull
if (-not $repoState.Exists) {
    Write-Step "Opprettar $repoFull"
    $create = Invoke-NativeProbe -FilePath "gh.exe" -Arguments @(
        "repo", "create", $repoFull, "--public",
        "--description", "Signed release metadata and installers for Nivra",
        "--disable-issues", "--disable-wiki"
    )

    if ($create.Output) { Write-Host $create.Output }
    if ($create.ExitCode -ne 0) {
        # Handle a race or a previous partially completed attempt: re-check before failing.
        $repoState = Test-GitHubRepository -NameWithOwner $repoFull
        if (-not $repoState.Exists) {
            throw "Klarte ikkje å opprette GitHub-repositoryet.`n$($create.Output)"
        }
    } else {
        $repoState = Test-GitHubRepository -NameWithOwner $repoFull
        if (-not $repoState.Exists) {
            throw "GitHub rapporterte at opprettinga var ferdig, men repositoryet kunne ikkje finnast etterpå."
        }
    }
} else {
    Write-Host "Repositoryet finst allereie: https://github.com/$repoFull" -ForegroundColor DarkGray
}

Write-Step "Klargjer lokal release-mappe"
New-Item -ItemType Directory -Force -Path $InstallDirectory | Out-Null

$packageRootFull = [IO.Path]::GetFullPath($packageRoot).TrimEnd('\')
$installFull = [IO.Path]::GetFullPath($InstallDirectory).TrimEnd('\')
if (-not [StringComparer]::OrdinalIgnoreCase.Equals($packageRootFull, $installFull)) {
    Get-ChildItem -LiteralPath $packageRoot -Force |
        Where-Object { $_.Name -notin @("dist", ".git") } |
        ForEach-Object {
            Copy-Item -LiteralPath $_.FullName -Destination $InstallDirectory -Recurse -Force
        }
} else {
    Write-Host "Oppsettet køyrer allereie frå den permanente release-mappa." -ForegroundColor DarkGray
}

Push-Location $InstallDirectory
try {
    if (-not (Test-Path ".git")) {
        $init = Invoke-NativeProbe -FilePath "git.exe" -Arguments @("init", "-b", "main")
        if ($init.ExitCode -ne 0) {
            $init = Invoke-NativeProbe -FilePath "git.exe" -Arguments @("init")
            if ($init.ExitCode -ne 0) { throw "git init feila.`n$($init.Output)" }
            Invoke-NativeRequired -FilePath "git.exe" -Arguments @("symbolic-ref", "HEAD", "refs/heads/main") -FailureMessage "Klarte ikkje å setje Git-branchen til main"
        }
        if ($init.Output) { Write-Host $init.Output }
    }

    # Repair branch state left by older or interrupted setup attempts.
    $head = Invoke-NativeProbe -FilePath "git.exe" -Arguments @("rev-parse", "--verify", "HEAD")
    if ($head.ExitCode -eq 0) {
        Invoke-NativeRequired -FilePath "git.exe" -Arguments @("branch", "-M", "main") -FailureMessage "Klarte ikkje å bruke main-branchen"
    } else {
        Invoke-NativeRequired -FilePath "git.exe" -Arguments @("symbolic-ref", "HEAD", "refs/heads/main") -FailureMessage "Klarte ikkje å klargjere main-branchen"
    }

    Invoke-NativeRequired -FilePath "git.exe" -Arguments @("config", "user.name", "Nivra Release Bot") -FailureMessage "git config user.name feila"
    $gitEmail = if ($userId) { "$userId+$login@users.noreply.github.com" } else { "$login@users.noreply.github.com" }
    Invoke-NativeRequired -FilePath "git.exe" -Arguments @("config", "user.email", $gitEmail) -FailureMessage "git config user.email feila"

    $remote = Invoke-NativeProbe -FilePath "git.exe" -Arguments @("remote", "get-url", "origin")
    if ($remote.ExitCode -ne 0) {
        Invoke-NativeRequired -FilePath "git.exe" -Arguments @("remote", "add", "origin", $expectedRemote) -FailureMessage "Klarte ikkje å leggje til GitHub-remote"
    } elseif ($remote.Output.Trim() -ne $expectedRemote) {
        Write-Warning "Origin peika på '$($remote.Output.Trim())'. Han blir retta til '$expectedRemote'."
        Invoke-NativeRequired -FilePath "git.exe" -Arguments @("remote", "set-url", "origin", $expectedRemote) -FailureMessage "Klarte ikkje å rette GitHub-remote"
    }

    Write-Step "Opprettar eller lastar release-signering"
    Invoke-NativeRequired -FilePath ".\tools\NivraRelease.exe" -Arguments @("keygen", "--site", ".\site") -FailureMessage "Klarte ikkje å opprette signeringsnøkkel"

    $backupPath = Join-Path ([Environment]::GetFolderPath("MyDocuments")) "Nivra-Release-Signing-Key-Backup.nivrakey"
    if (Test-Path -LiteralPath $backupPath) {
        $backupChoice = Read-Host "Ein recovery-backup finst allereie. Lage ein ny og erstatte fila? [j/N]"
        $makeBackup = ($backupChoice -match '^[JjYy]')
    } else {
        $backupChoice = Read-Host "Opprett ein passordkryptert recovery-backup av signeringsnøkkelen no? [J/n]"
        $makeBackup = ($backupChoice -notmatch '^[Nn]')
    }

    if ($makeBackup) {
        function Convert-SecureToPlain([Security.SecureString]$Secure) {
            $ptr = [Runtime.InteropServices.Marshal]::SecureStringToBSTR($Secure)
            try { return [Runtime.InteropServices.Marshal]::PtrToStringBSTR($ptr) }
            finally { [Runtime.InteropServices.Marshal]::ZeroFreeBSTR($ptr) }
        }

        $first = Read-Host "Vel eit recovery-passord på minst 12 teikn" -AsSecureString
        $second = Read-Host "Skriv recovery-passordet ein gong til" -AsSecureString
        $plainFirst = Convert-SecureToPlain $first
        $plainSecond = Convert-SecureToPlain $second
        try {
            if ($plainFirst -cne $plainSecond) { throw "Recovery-passorda var ikkje like." }
            if ($plainFirst.Length -lt 12) { throw "Recovery-passordet må ha minst 12 teikn." }
            $env:NIVRA_RELEASE_BACKUP_PASSPHRASE = $plainFirst
            Invoke-NativeRequired -FilePath ".\tools\NivraRelease.exe" -Arguments @("backup-key", "--out", $backupPath) -FailureMessage "Backup av signeringsnøkkelen feila"
            Write-Host "Recovery-backup: $backupPath" -ForegroundColor Yellow
            Write-Host "Kopier denne fila til ein trygg plass, og ikkje mist passordet." -ForegroundColor Yellow
        } finally {
            Remove-Item Env:NIVRA_RELEASE_BACKUP_PASSPHRASE -ErrorAction SilentlyContinue
            $plainFirst = $null
            $plainSecond = $null
        }
    } else {
        Write-Warning "Utan recovery-backup kan framtidige oppdateringar ikkje signerast dersom Windows-kontoen eller PC-en går tapt."
    }

    Invoke-NativeRequired -FilePath "git.exe" -Arguments @("add", "--all") -FailureMessage "git add feila"
    $staged = Invoke-NativeProbe -FilePath "git.exe" -Arguments @("diff", "--cached", "--quiet")
    if ($staged.ExitCode -eq 1) {
        Invoke-NativeRequired -FilePath "git.exe" -Arguments @("commit", "-m", "chore: initialize or repair Nivra release service") -FailureMessage "git commit feila"
    } elseif ($staged.ExitCode -ne 0) {
        throw "Klarte ikkje å kontrollere Git-endringane.`n$($staged.Output)"
    } else {
        Write-Host "Ingen nye lokale release-filer å committe." -ForegroundColor DarkGray
    }

    Write-Step "Lastar release-tenesta opp til GitHub"
    $push = Invoke-NativeProbe -FilePath "git.exe" -Arguments @("push", "--set-upstream", "origin", "main")
    if ($push.Output) { Write-Host $push.Output }
    if ($push.ExitCode -ne 0) {
        throw "git push feila. Repositoryet blei oppretta, men filene blei ikkje lasta opp.`n$($push.Output)"
    }

    Write-Step "Aktiverer GitHub Pages"
    $pagesCreate = Invoke-NativeProbe -FilePath "gh.exe" -Arguments @(
        "api", "--method", "POST", "repos/$repoFull/pages", "-f", "build_type=workflow"
    )

    if ($pagesCreate.ExitCode -ne 0) {
        $pagesRead = Invoke-NativeProbe -FilePath "gh.exe" -Arguments @("api", "repos/$repoFull/pages", "--silent")
        if ($pagesRead.ExitCode -ne 0) {
            throw "GitHub Pages kunne ikkje aktiverast automatisk. Opne repository Settings -> Pages og vel GitHub Actions.`n$($pagesCreate.Output)`n$($pagesRead.Output)"
        }

        $pagesUpdate = Invoke-NativeProbe -FilePath "gh.exe" -Arguments @(
            "api", "--method", "PUT", "repos/$repoFull/pages",
            "-f", "build_type=workflow", "-F", "https_enforced=true"
        )
        if ($pagesUpdate.ExitCode -ne 0) {
            Write-Warning "Pages finst, men innstillingane kunne ikkje oppdaterast automatisk: $($pagesUpdate.Output)"
        }
    }

    # The initial push normally triggers Pages. A manual dispatch is retried because
    # GitHub can need a few seconds before a newly pushed workflow is discoverable.
    $workflowStarted = $false
    for ($attempt = 1; $attempt -le 6; $attempt++) {
        $workflow = Invoke-NativeProbe -FilePath "gh.exe" -Arguments @(
            "workflow", "run", "pages.yml", "--repo", $repoFull
        )
        if ($workflow.ExitCode -eq 0) {
            $workflowStarted = $true
            break
        }
        Start-Sleep -Seconds 3
    }
    if (-not $workflowStarted) {
        Write-Warning "Kunne ikkje starte Pages-workflowen manuelt. Push-hendinga kan framleis ha starta han automatisk. Kontroller Actions-fana på GitHub."
    }

    $pageUrl = "https://$Owner.github.io/$Repository/"
    Write-Host @"

Release-tenesta er konfigurert.

Repository: https://github.com/$repoFull
Statusside:  $pageUrl
Alpha-feed: ${pageUrl}channels/alpha/windows-x64.json

Den private signeringsnøkkelen er beskytta med Windows DPAPI og ligg berre
på denne Windows-kontoen. Ikkje slett mappa under LOCALAPPDATA\Nivra\Release.

GitHub Actions brukar vanlegvis nokre minutt på den første publiseringa.
"@ -ForegroundColor Green

    Start-Process $pageUrl
} finally {
    Pop-Location
}
