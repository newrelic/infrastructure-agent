<#
    .SYNOPSIS
        This script packages the New Relic Infrastructure Agent as a zip file
#>
param (
    # Target architecture: amd64 (default) or 386
    [ValidateSet("amd64", "386")]
    [string]$arch="amd64",
    [string]$version="0.0.0"
)

Write-Output "Building zip package"

$scriptPath = split-path -parent $MyInvocation.MyCommand.Definition
$workspace = "$scriptPath\..\.."

New-Item -path "$scriptPath\target\newrelic-infra\Program Files\New Relic\newrelic-infra\custom-integrations" -type directory -Force
New-Item -path "$scriptPath\target\newrelic-infra\Program Files\New Relic\newrelic-infra\integrations.d" -type directory -Force
New-Item -path "$scriptPath\target\newrelic-infra\Program Files\New Relic\newrelic-infra\newrelic-integrations" -type directory -Force

Copy-Item -Path "$workspace\target\bin\windows_$arch\newrelic-infra.exe" -Destination "$scriptPath\target\newrelic-infra\Program Files\New Relic\newrelic-infra\"
Copy-Item -Path "$workspace\target\bin\windows_$arch\newrelic-infra-ctl.exe" -Destination "$scriptPath\target\newrelic-infra\Program Files\New Relic\newrelic-infra\"
Copy-Item -Path "$workspace\target\bin\windows_$arch\newrelic-infra-service.exe" -Destination "$scriptPath\target\newrelic-infra\Program Files\New Relic\newrelic-infra\"
Copy-Item -Path "$workspace\target\bin\windows_$arch\yamlgen.exe" -Destination "$scriptPath\target\newrelic-infra\Program Files\New Relic\newrelic-infra\"
Copy-Item -Path "$workspace\assets\examples\infrastructure\LICENSE.windows.txt" -Destination "$scriptPath\target\newrelic-infra\Program Files\New Relic\newrelic-infra\LICENSE.txt"
Copy-Item -Path "$workspace\build\package\binaries\windows\installer.ps1" -Destination "$scriptPath\target\newrelic-infra\Program Files\New Relic\newrelic-infra\installer.ps1"

New-Item -path "$workspace\dist" -type directory -Force
Compress-Archive -Path "$scriptPath\target\newrelic-infra\Program Files" -DestinationPath "$workspace\dist\newrelic-infra-$arch.$version.zip" -Force
Remove-Item "$scriptPath\target" -Force -Recurse