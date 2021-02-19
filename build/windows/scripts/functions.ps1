<#
    .SYNOPSIS
        This script contains common functions for building the Windows New Relic Infrastructure Agent.
#>
Function SignExecutable {
    param (
        # Signing tool
        [string]$signtool='"C:\Program Files (x86)\Windows Kits\10\bin\x64\signtool.exe"',
        [string]$executable=$(throw "-executable path is required")
    )

    Invoke-Expression "& $signtool sign /d 'New Relic Infrastructure Agent' /n 'New Relic, Inc.' $executable"
    if ($lastExitCode -ne 0) {
       throw "Failed to sign $executable"
    }
}

Function GetIntegrationVersion {
    param (
        [string]$name=$(throw "-name is required")
    )
    $dir = "$scriptPath\..\..\embed"

    [string]$version=$(Get-Content "$dir\integrations.version" | %{if($_ -match "^$name") { $_.Split(',')[1]; }})
    
    if ([string]::IsNullOrWhitespace($version)) {
        throw "failed to read $name version"
    }

    return $version
}

Function GetFluentBitVersion {
    $versionsArray = @()

    $pluginVersion = $(Get-Content $dir/fluent-bit.version | %{if($_ -match "^windows") { $_.Split(',')[1]; }})
    if ([string]::IsNullOrWhitespace($pluginVersion)) {
        throw "failed to read nr fluent-bit plugin version"
    }
    $versionsArray += $pluginVersion

    $dir = "$scriptPath\..\..\embed"
    $fbVersion = $(Get-Content $dir/fluent-bit.version | %{if($_ -match "^windows") { $_.Split(',')[2]; }})
    if ([string]::IsNullOrWhitespace($fbVersion)) {
        throw "failed to read nr fluent-bit version"
    }
    $versionsArray += $fbVersion

    return ,$versionsArray
}

Function DownloadFile {
    param (
        [string]$url=$(throw "-url is required"),
        [string]$dest=$(throw "-dest is required")
    )

    # $extractDir = (Get-Item $dest).Directory.Name
    $file = $url.Substring($url.LastIndexOf("/") + 1)

    # Download zip file.
    $ProgressPreference = 'SilentlyContinue'
    [Net.ServicePointManager]::SecurityProtocol = [Net.SecurityProtocolType]::Tls12

    Write-Output "Downloading $url"

    New-Item -path $dest -type directory -Force
    Invoke-WebRequest $url -OutFile "$dest\$file"
}

Function DownloadAndExtractZip {
    param (
        [string]$url=$(throw "-url is required"),
        [string]$dest=$(throw "-dest is required")
    )

    DownloadFile -dest:"$dest" -url:"$url"

    # extract
    expand-archive -path "$dest\$file" -destinationpath $dest
    Remove-Item "$dest\$file"
}