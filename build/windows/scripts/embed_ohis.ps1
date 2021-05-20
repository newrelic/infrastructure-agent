<#
    .SYNOPSIS
        This script downloads all the embeded integrations for New Relic Infrastructure Agent
#>
param (
    # Target architecture: amd64 (default) or 386
    [ValidateSet("amd64", "386")]
    [string]$arch="amd64",
    [string]$scriptPath=$(split-path -parent $MyInvocation.MyCommand.Definition),

    # Skip signing
    [switch]$skipSigning=$false,
    # Signing tool
    [string]$signtool='"C:\Program Files (x86)\Windows Kits\10\bin\x64\signtool.exe"'
)

# Source build Functions.
. $scriptPath/functions.ps1

# Adding flex.
Function EmbedFlex {
    Write-Output "--- Embedding nri-flex"

    [string]$version = GetIntegrationVersion -name "nri-flex"
    [string]$url="https://github.com/newrelic/nri-flex/releases/download/v${version}/nri-flex_windows_${version}_${arch}.zip"

    DownloadAndExtractZip -dest:"$downloadPath\nri-flex" -url:"$url"

    if (-Not $skipSigning) {
        SignExecutable -executable "$downloadPath\nri-flex\nri-flex.exe"
    }
}

# # Adding windows services.
Function EmbedWindowsServices {
    Write-Output "--- Embedding win-services"
    [string]$version = GetIntegrationVersion -name "nri-winservices"

    # download
    [string]$file="nri-winservices-${version}-amd64.zip" # TODO change this with $arch when package is available.

    [string]$url="https://github.com/newrelic/nri-winservices/releases/download/${version}/${file}"

    DownloadAndExtractZip -dest:"$downloadPath\nri-winservices" -url:"$url"

    if (-Not $skipSigning) {
        SignExecutable -executable "$downloadPath\nri-winservices\nri-winservices.exe"
        SignExecutable -executable "$downloadPath\nri-winservices\windows_exporter.exe"
    }
}

# embded nri-prometheus
Function EmbedPrometheus {
    Write-Output "--- Embedding nri-prometheus"

    [string]$version = GetIntegrationVersion -name "nri-prometheus"

    # download
    [string]$file="nri-prometheus-$arch.${version}.zip"
    $url="https://github.com/newrelic/nri-prometheus/releases/download/v${version}/${file}"
 
    DownloadAndExtractZip -dest:"$downloadPath\nri-prometheus" -url:"$url"

    Copy-Item -Path "$downloadPath\nri-prometheus\New Relic\newrelic-infra\newrelic-integrations\bin\nri-prometheus.exe" -Destination "$downloadPath\nri-prometheus\nri-prometheus.exe" -Recurse -Force
    Remove-Item -Path "$downloadPath\nri-prometheus\New Relic" -Force -Recurse

    if (-Not $skipSigning) {
        SignExecutable -executable "$downloadPath\nri-prometheus\nri-prometheus.exe"
    }
}

# embded fluent-bit
Function EmbedFluentBit {
    Write-Output "--- Embedding fluent-bit"

    $pluginVersion = GetFluentBitPluginVersion
    $nrfbVersion = GetFluentBitVersion

    [string]$pluginUrl = "https://github.com/newrelic/newrelic-fluent-bit-output/releases/download/v$pluginVersion/out_newrelic-windows-$arch-$pluginVersion.dll"
    DownloadFile -dest:"$downloadPath\logging\nrfb" -outFile:"out_newrelic.dll" -url:"$pluginUrl"

    [string]$nrfbUrl = "https://github.com/newrelic-experimental/fluent-bit-package/releases/download/$nrfbVersion/fb-windows-$arch.zip"
    DownloadAndExtractZip -dest:"$downloadPath\logging\nrfb" -url:"$nrfbUrl"
    
    if (-Not $skipSigning) {
        SignExecutable -executable "$downloadPath\logging\nrfb\fluent-bit.exe"
    }
}

Function EmbedWinpkg {
    Write-Output "===> Embeding Winpkg $arch"

    $UrlPath = "windows/integrations/nri-winpkg/nri-winpkg.zip"
    if($arch -eq "386") {
        $UrlPath = "windows/386/integrations/nri-winpkg/nri-winpkg_386.zip"
    }

    [string]$url = "https://download.newrelic.com/infrastructure_agent/$UrlPath"
    DownloadAndExtractZip -dest:"$downloadPath" -url:"$url"
    
    if (-Not $skipSigning) {
        SignExecutable -executable "$downloadPath\winpkg\nr-winpkg.exe"
    }
}

# Call all the steps.
$downloadPath = "$scriptPath\..\..\..\target\embed\bin\windows_$arch\"

Write-Output "--- Cleaning..."

Remove-Item $downloadPath -Recurse -ErrorAction Ignore
New-Item -ItemType Directory -Force -Path "$downloadPath"

Write-Output "--- Embedding external components"

EmbedFlex
EmbedWindowsServices
EmbedPrometheus
EmbedFluentBit
EmbedWinpkg

exit 0