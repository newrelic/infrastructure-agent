<#
    .SYNOPSIS
        This script downloads all the embeded integrations for New Relic Infrastructure Agent
#>
param (
    # Target architecture: amd64 (default) or 386
    [ValidateSet("amd64", "386")]
    [string]$arch="amd64",
    [string]$scriptPath=$(split-path -parent $MyInvocation.MyCommand.Definition),
    # nri-flex
    [string]$nriFlexVersion=$(Get-Content $scriptPath/integrations.version | %{if($_ -match "^nri-flex") { $_.Split(',')[1]; }}),
    # nri-windowsservice
    [string]$nriWinServicesVersion=$(Get-Content $scriptPath/integrations.version | %{if($_ -match "^nri-winservices") { $_.Split(',')[1]; }}),
    # nri-prometheus
    [string]$nriPrometheusVersion=$(Get-Content $scriptPath/integrations.version | %{if($_ -match "^nri-prometheus") { $_.Split(',')[1]; }}),
    #fluent-bit
    [string]$nrfbArtifactVersion=$(Get-Content $scriptPath/fluent-bit.version | %{if($_ -match "^windows") { $_.Split(',')[3]; }}),

    # Skip signing
    [switch]$skipSigning=$false,
    # Signing tool
    [string]$signtool='"C:\Program Files (x86)\Windows Kits\10\bin\x64\signtool.exe"'
    )

$downloadPath = "$scriptPath\..\..\target\embed"

Write-Output "--- Cleaning..."

Remove-Item $downloadPath -Recurse -ErrorAction Ignore
New-Item -ItemType Directory -Force -Path "$downloadPath\bin\windows_$arch\"

Write-Output "--- Embedding external components"
# embded flex
if (-Not [string]::IsNullOrWhitespace($nriFlexVersion)) {
    # download
    Write-Output "--- Embedding nri-flex ${nriFlexVersion}"

    [string]$release="v${nriFlexVersion}"
    [string]$file="nri-flex_${nriFlexVersion}_Windows_x86_64.zip"

    $ProgressPreference = 'SilentlyContinue'
    [Net.ServicePointManager]::SecurityProtocol = [Net.SecurityProtocolType]::Tls12
    Invoke-WebRequest "https://github.com/newrelic/nri-flex/releases/download/${release}/${file}" -OutFile "$downloadPath\nri-flex.zip"

    # extract
    $dstPath = "$downloadPath\bin\windows_$arch\nri-flex"
    New-Item -path $dstPath -type directory -Force
    expand-archive -path "$downloadPath\nri-flex.zip" -destinationpath $dstPath
    Remove-Item "$downloadPath\nri-flex.zip"

    if (-Not $skipSigning) {
        Invoke-Expression "& $signtool sign /d 'New Relic Infrastructure Agent' /n 'New Relic, Inc.'  $dstPath\nri-flex.exe"
        if ($lastExitCode -ne 0) {
            Write-Output "Failed to sign flex"
            exit -1
        }
    }
}
# embded nri-winservices
if (-Not [string]::IsNullOrWhitespace($nriWinServicesVersion)) {
    Write-Output "--- Embedding win-services"

    # download
    [string]$file="nri-winservices-${nriWinServicesVersion}-$arch.zip"
    $ProgressPreference = 'SilentlyContinue'
    [Net.ServicePointManager]::SecurityProtocol = [Net.SecurityProtocolType]::Tls12
    Invoke-WebRequest "https://github.com/newrelic/nri-winservices/releases/download/${nriWinServicesVersion}/${file}" -OutFile "$downloadPath\nri-winservices.zip"

    # extract
    $windowsTargetPath = "$downloadPath\bin\windows_$arch\nri-winservices"
    New-Item -path $windowsTargetPath -type directory -Force
    expand-archive -path "$downloadPath\nri-winservices.zip" -destinationpath $windowsTargetPath
    Remove-Item "$downloadPath\nri-winservices.zip"

    if (-Not $skipSigning) {
        Invoke-Expression "& $signtool sign /d 'New Relic Infrastructure Agent' /n 'New Relic, Inc.'  $windowsTargetPath\nri-winservices.exe"
        if ($lastExitCode -ne 0) {
            Write-Output "Failed to sign winservices"
            exit -1
        }
        Invoke-Expression "& $signtool sign /d 'New Relic Infrastructure Agent' /n 'New Relic, Inc.'  $windowsTargetPath\windows_exporter.exe"
        if ($lastExitCode -ne 0) {
            Write-Output "Failed to sign win services exported"
            exit -1
        }
    }
}

# embded nri-prometheus
if (-Not [string]::IsNullOrWhitespace($nriPrometheusVersion)) {
    Write-Output "--- Embedding nri-prometheus"

    # download
    $ProgressPreference = 'SilentlyContinue'
    [Net.ServicePointManager]::SecurityProtocol = [Net.SecurityProtocolType]::Tls12
    [string]$file="nri-prometheus-$arch.${nriPrometheusVersion}.zip"
    $prometheusUrl="https://github.com/newrelic/nri-prometheus/releases/download/v${nriPrometheusVersion}/${file}"
    
    Write-Output "--- Downloading $prometheusUrl"
    Invoke-WebRequest $prometheusUrl -OutFile "$downloadPath\nri-prometheus.zip"
    
    # extract
    $prometheusPath = "$downloadPath\nri-prometheus"
    $dstPath = "$downloadPath\bin\windows_$arch\nri-prometheus"

    New-Item -path $prometheusPath -type directory -Force
    New-Item -path "$dstPath" -type directory -Force

    expand-archive -path "$downloadPath\nri-prometheus.zip" -destinationpath $prometheusPath
    Remove-Item "$downloadPath\nri-prometheus.zip"

    Copy-Item -Path "$prometheusPath\New Relic\newrelic-infra\newrelic-integrations\bin\nri-prometheus.exe" -Destination "$dstPath\nri-prometheus.exe" -Recurse -Force

    if (-Not $skipSigning) {
        Invoke-Expression "& $signtool sign /d 'New Relic Infrastructure Agent' /n 'New Relic, Inc.' $dstPath\nri-prometheus.exe"
        if ($lastExitCode -ne 0) {
            Write-Output "Failed to sign prometheus"
            exit -1
        }
    }
    Remove-Item -Path $prometheusPath -Force -Recurse
}

# embded fluent-bit
if (-Not [string]::IsNullOrWhitespace($nrfbArtifactVersion)) {

    Write-Output "--- Embedding fluent-bit"

    $fbArch = "win64"
    if($arch -eq "386") {
        $fbArch = "win32"
    }
    # Download fluent-bit artifacts.
    $ProgressPreference = 'SilentlyContinue'
    [Net.ServicePointManager]::SecurityProtocol = [Net.SecurityProtocolType]::Tls12
    Invoke-WebRequest "https://download.newrelic.com/infrastructure_agent/logging/windows/nrfb-$nrfbArtifactVersion-$fbArch.zip" -Headers @{"X-JFrog-Art-Api"="$artifactoryToken"} -OutFile nrfb.zip

    expand-archive -path '.\nrfb.zip' -destinationpath '.\'
    Remove-Item -Force .\nrfb.zip

    if (-Not $skipSigning) {
        Invoke-Expression "& $signtool sign /d 'New Relic Infrastructure Agent' /n 'New Relic, Inc.'  .\nrfb\fluent-bit.exe"
        if ($lastExitCode -ne 0) {
            Write-Output "Failed to sign fluent-bit"
            exit -1
        }
    }

    # Move the files to packaging.
    $nraPath = "$downloadPath\bin\windows_$arch\"
    New-Item -path "$nraPath\logging" -type directory -Force
    Copy-Item -Path ".\nrfb\*" -Destination "$nraPath\logging" -Recurse -Force
    
    Remove-Item -Path ".\nrfb" -Force -Recurse
}

Write-Output "===> Embeding Winpkg $arch"

$UrlPath = "windows/integrations/nri-winpkg/nri-winpkg.zip"
if($arch -eq "386") {
    $UrlPath = "windows/386/integrations/nri-winpkg/nri-winpkg_386.zip"
}

# Download WinPkg artifacts.
$ProgressPreference = 'SilentlyContinue'
[Net.ServicePointManager]::SecurityProtocol = [Net.SecurityProtocolType]::Tls12
Invoke-WebRequest "https://download.newrelic.com/infrastructure_agent/$UrlPath" -OutFile "$downloadPath\nri-winpkg.zip"

# extract
$dstPath = "$downloadPath\bin\windows_$arch\"
New-Item -path $dstPath -type directory -Force
expand-archive -path "$downloadPath\nri-winpkg.zip" -destinationpath $dstPath
Remove-Item "$downloadPath\nri-winpkg.zip"

if (-Not $skipSigning) {
    Invoke-Expression "& $signtool sign /d 'New Relic Infrastructure Agent' /n 'New Relic, Inc.'  $dstPath\nr-winpkg.exe"
    if ($lastExitCode -ne 0) {
        Write-Output "Failed to sign winpkg"
        exit -1
    }
}

