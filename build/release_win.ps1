<#
    .SYNOPSIS
        This script creates the Newrelic Infrastructure Agent msi package.
#>
param (
    # Target architecture: amd64 (default) or 386
    [string]$integration="none",
    [ValidateSet("amd64", "386")]
    [string]$arch="amd64",
    [string]$version="0.0.0",

    # Skip signing
    [switch]$skipSigning=$false
)

$scriptPath = split-path -parent $MyInvocation.MyCommand.Definition

$buildYear = (Get-Date).Year

Write-Output "===> Embeding integrations"

Invoke-expression -Command "$scriptPath\embed\integrations_win.ps1 -arch $arch $(If ($skipSigning) {"-skipSigning"} Else {"false"})"
if ($lastExitCode -ne 0) {
    Write-Output "Failed to embed integration"
    exit -1
}

Write-Output "===> Checking MSBuild.exe..."
$msBuild = (Get-ItemProperty hklm:\software\Microsoft\MSBuild\ToolsVersions\4.0).MSBuildToolsPath
if ($msBuild.Length -eq 0) {
    Write-Output "Can't find MSBuild tool. .NET Framework 4.0.x must be installed"
    exit -1
}
Write-Output $msBuild

Write-Output "===> Building msi Installer"

$env:path = "$env:path;C:\Program Files (x86)\Microsoft Visual Studio\2019\Enterprise\MSBuild\Current\Bin"

$WixPrjPath = "$scriptPath\package\windows\newrelic-infra-$arch-installer\newrelic-infra"
. $msBuild/MSBuild.exe "$WixPrjPath\newrelic-infra-installer.wixproj" /p:AgentVersion=${version}

if (-not $?)
{
    Write-Output "Failed building installer"
    exit -1
}

Write-Output "===> Making versioned installed copy"
Copy-Item $WixPrjPath\bin\Release\newrelic-infra-$arch.msi $WixPrjPath\bin\Release\newrelic-infra-${arch}.${version}.msi

exit 0