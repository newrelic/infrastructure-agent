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
$workspace = "$scriptPath\..\.."

$buildYear = (Get-Date).Year

Write-Output "===> Embeding integrations"
Invoke-expression -Command "$scriptPath\scripts\embed_ohis.ps1 -arch $arch $(If ($skipSigning) {"-skipSigning"})"
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

$env:path = "$env:path;C:\Program Files\Microsoft Visual Studio\2022\Enterprise\MSBuild\Current\Bin"
$WixPrjPath = "$scriptPath\..\package\windows\newrelic-infra-$arch-installer\newrelic-infra"
. $msBuild/MSBuild.exe "$WixPrjPath\newrelic-infra-installer.wixproj" /p:AgentVersion=${version} /p:Year=$buildYear /p:SkipSigning=${skipSigning}

if (-not $?)
{
    Write-Output "Failed building installer"
    exit -1
}

Write-Output "===> Making versioned installed copy"

New-Item -path "$workspace\dist" -type directory -Force
Copy-Item $WixPrjPath\bin\Release\newrelic-infra-$arch.msi "$workspace\dist\newrelic-infra-${arch}.${version}.msi"

exit 0