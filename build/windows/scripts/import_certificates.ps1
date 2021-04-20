<#
    .SYNOPSIS
        This script imports the Newrelic Infrastructure Agent certificate.
#>
param (
    [string]$pfx_passphrase="none",
    [string]$pfx_certificate_description="none"
)

Write-Output "===> Import .pfx certificate from GH Secrets"
Import-PfxCertificate -FilePath wincert.pfx -Password (ConvertTo-SecureString -String $pfx_passphrase -AsPlainText -Force) -CertStoreLocation Cert:\CurrentUser\My

Write-Output "===> Show certificate installed"
Get-ChildItem -Path cert:\CurrentUser\My\

exit 0