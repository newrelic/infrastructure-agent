param(
    [string]$PfxPassphrase,
    [string]$PfxCertificateDescription
)

$rawOutput = build\windows\scripts\import_certificates.ps1 -pfx_passphrase $PfxPassphrase -pfx_certificate_description $PfxCertificateDescription

# Extract the Thumbprint value, ensuring only the first instance is captured
$thumbprint = ($rawOutput | Select-String -Pattern '\[Thumbprint\]\s*\n\s*([0-9A-F]{40})' | Select-Object -First 1 | ForEach-Object { $_.Matches.Groups[1].Value }).Trim()
Write-Host "Thumbprint: $thumbprint"

# Set the thumbprint as an environment variable for GitHub Actions
"certThumbprint=$thumbprint" | Out-File -FilePath $env:GITHUB_ENV -Encoding utf8 -Append