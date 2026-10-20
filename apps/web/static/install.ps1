# Usage: irm https://YOUR-PEEP-HOST/install.ps1 | iex
# Or save this file, inspect it, then run: .\install.ps1
# Installs the latest Windows x64 Tauri release. The app asks for its API URL.
# No execution-policy or signature checks are bypassed.
[CmdletBinding()]
param()
$ErrorActionPreference = 'Stop'

if ($env:OS -ne 'Windows_NT') {
    throw 'This installer is for Windows only.'
}
if (-not [Environment]::Is64BitOperatingSystem -or $env:PROCESSOR_ARCHITECTURE -eq 'ARM64' -or $env:PROCESSOR_ARCHITEW6432 -eq 'ARM64') {
    throw 'This release installer requires an x64 Windows system.'
}
foreach ($command in 'Invoke-RestMethod', 'Invoke-WebRequest', 'Get-AuthenticodeSignature', 'Start-Process') {
    if (-not (Get-Command $command -ErrorAction SilentlyContinue)) {
        throw "Missing required PowerShell command: $command"
    }
}
# Windows PowerShell 5.1 may otherwise negotiate obsolete TLS versions.
[Net.ServicePointManager]::SecurityProtocol = [Net.ServicePointManager]::SecurityProtocol -bor [Net.SecurityProtocolType]::Tls12
$tempDirectory = Join-Path ([IO.Path]::GetTempPath()) ('peep-install-' + [guid]::NewGuid().ToString('N'))
try {
    $release = Invoke-RestMethod -Uri 'https://api.github.com/repos/EaeDave/peep/releases/latest' -Headers @{ 'User-Agent' = 'Peep-Installer' } -UseBasicParsing
    $assets = @($release.assets | Where-Object { $_.name -match '(?i)_x64[-_]setup\.exe$' })
    if ($assets.Count -eq 0) {
        $assets = @($release.assets | Where-Object {
            $_.name -match '(?i)-setup\.exe$' -and $_.name -notmatch '(?i)(arm|aarch|i[3-6]86|ia32|x86(?!_64))'
        })
    }
    if ($assets.Count -ne 1) {
        throw 'The latest release must contain exactly one Windows x64 NSIS setup executable.'
    }
    $download = [uri]$assets[0].browser_download_url
    if ($download.Scheme -ne 'https' -or $download.Host -ne 'github.com' -or -not $download.AbsolutePath.StartsWith('/EaeDave/peep/releases/download/', [StringComparison]::OrdinalIgnoreCase)) {
        throw 'The release returned an unexpected download URL.'
    }
    New-Item -ItemType Directory -Path $tempDirectory | Out-Null
    $installer = Join-Path $tempDirectory 'peep-setup.exe'
    Invoke-WebRequest -Uri $download -OutFile $installer -UseBasicParsing
    # Preserve the Windows Internet-zone trust boundary for the downloaded executable.
    Set-Content -LiteralPath $installer -Stream Zone.Identifier -Value "[ZoneTransfer]`r`nZoneId=3" -Encoding ASCII
    $signature = Get-AuthenticodeSignature -LiteralPath $installer
    if ($signature.Status -eq 'NotSigned') {
        Write-Warning 'This Peep release is unsigned. Windows cannot verify its publisher. Only continue if you trust EaeDave/peep and this release.'
        if ((Read-Host 'Run the unsigned installer? Type YES to continue') -cne 'YES') {
            throw 'Installation cancelled. The unsigned executable was not run.'
        }
    } elseif ($signature.Status -ne 'Valid') {
        throw "Windows rejected the installer signature ($($signature.Status)). The executable was not run."
    }
    $process = Start-Process -FilePath $installer -PassThru -Wait
    if ($process.ExitCode -notin @(0, 1641, 3010)) {
        throw "Peep setup did not complete successfully (exit code $($process.ExitCode))."
    }
    Write-Host 'Peep setup completed. Open Peep and configure your API URL and invitation locally. Existing client configuration was not modified.'
} finally {
    if (Test-Path -LiteralPath $tempDirectory) {
        Remove-Item -LiteralPath $tempDirectory -Recurse -Force
    }
}
