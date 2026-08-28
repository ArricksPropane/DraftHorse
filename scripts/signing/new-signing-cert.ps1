#Requires -Version 5.1
<#
.SYNOPSIS
  One-time: generate the Arrick's Propane self-signed code-signing certificate
  and export everything the pipeline and the fleet need.

.DESCRIPTION
  Run ONCE, on any Windows machine, as the person who will own the key.
  Produces three files in the current directory:

    DraftHorse-signing.pfx      private key, password-protected. ARCHIVE THIS
                                OFFLINE with the release archive; anyone with
                                it + the password can sign as Arrick's Propane
                                for every machine that trusts the cert.
    DraftHorse-signing.cer      PUBLIC certificate — safe to distribute; what
                                trust-signing-cert.ps1 installs on each fleet
                                machine via ScreenConnect.
    DraftHorse-signing.pfx.b64  base64 of the PFX — paste the file's CONTENTS
                                into the GitHub repo secret SELFSIGN_PFX_B64
                                (and the password into SELFSIGN_PFX_PASSWORD).

  The certificate is then REMOVED from this machine's store: after this script
  the private key exists only inside the PFX. Deliberate — no stray exportable
  key left on whichever PC happened to run this.

  Trust model: self-signed. Only machines where trust-signing-cert.ps1 has run
  trust it — that is the point (fleet-only blast radius, no CA, no renewals
  for 10 years). SmartScreen reputation is NOT provided by any self-signed
  cert; the once-per-machine "More info → Run anyway" on downloaded installers
  remains, which is acceptable for ScreenConnect-driven installs.
#>
[CmdletBinding()]
param(
    [string]$Subject = "CN=Arrick's Propane",
    [int]$ValidYears = 10
)
Set-StrictMode -Version Latest
$ErrorActionPreference = 'Stop'

$pwd1 = Read-Host -AsSecureString -Prompt 'Choose a PFX password (goes into the SELFSIGN_PFX_PASSWORD secret)'
$pwd2 = Read-Host -AsSecureString -Prompt 'Confirm the password'
$p1 = [Runtime.InteropServices.Marshal]::PtrToStringAuto([Runtime.InteropServices.Marshal]::SecureStringToBSTR($pwd1))
$p2 = [Runtime.InteropServices.Marshal]::PtrToStringAuto([Runtime.InteropServices.Marshal]::SecureStringToBSTR($pwd2))
if ($p1 -ne $p2) { throw 'Passwords do not match.' }
if ($p1.Length -lt 12) { throw 'Use at least 12 characters — this password protects the signing key.' }

Write-Host "Generating $Subject (RSA 3072, SHA-256, $ValidYears years)..." -ForegroundColor Cyan
$cert = New-SelfSignedCertificate `
    -Type CodeSigningCert `
    -Subject $Subject `
    -FriendlyName 'DraftHorse code signing' `
    -CertStoreLocation 'Cert:\CurrentUser\My' `
    -KeyAlgorithm RSA -KeyLength 3072 -HashAlgorithm SHA256 `
    -KeyExportPolicy Exportable `
    -KeyUsage DigitalSignature `
    -NotAfter (Get-Date).AddYears($ValidYears)

$pfx = Join-Path (Get-Location) 'DraftHorse-signing.pfx'
$cer = Join-Path (Get-Location) 'DraftHorse-signing.cer'
$b64 = Join-Path (Get-Location) 'DraftHorse-signing.pfx.b64'

Export-PfxCertificate -Cert $cert -FilePath $pfx -Password $pwd1 | Out-Null
Export-Certificate    -Cert $cert -FilePath $cer | Out-Null
[IO.File]::WriteAllText($b64, [Convert]::ToBase64String([IO.File]::ReadAllBytes($pfx)))

# Private key now lives only in the PFX.
Remove-Item -LiteralPath "Cert:\CurrentUser\My\$($cert.Thumbprint)" -Force

Write-Host ''
Write-Host "Thumbprint: $($cert.Thumbprint)" -ForegroundColor Green
Write-Host "Expires:    $($cert.NotAfter)"   -ForegroundColor Green
Write-Host ''
Write-Host 'Next steps:' -ForegroundColor Cyan
Write-Host '  1. GitHub repo -> Settings -> Secrets and variables -> Actions:'
Write-Host '       SELFSIGN_PFX_B64      = contents of DraftHorse-signing.pfx.b64'
Write-Host '       SELFSIGN_PFX_PASSWORD = the password you just chose'
Write-Host '  2. Archive DraftHorse-signing.pfx + password OFFLINE (with the release archive).'
Write-Host '  3. Distribute DraftHorse-signing.cer to fleet machines with trust-signing-cert.ps1.'
Write-Host '  4. Delete DraftHorse-signing.pfx and .b64 from this machine once archived.'
