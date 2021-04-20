<#
    .SYNOPSIS
        This script add metadata to windows exe.
#>
param (
	 [string]$version = "0.0.0"
)

$major = $version.Split(".")[0]
$minor = $version.Split(".")[1]
$patch = $version.Split(".")[2]
$build = 0
$buildYear = (Get-Date).Year

$scriptPath = split-path -parent $MyInvocation.MyCommand.Definition

$agentPath = "$scriptPath\..\..\.."

Function GenerateVersionInfoFile($exeName) {
  $versionInfoPath = Join-Path -Path $agentPath -ChildPath "cmd\$exeName\versioninfo.json"
  if ((Test-Path "$versionInfoPath.template" -PathType Leaf) -eq $False) {
    Write-Error "$versionInfoPath.template not found."
  }
  Copy-Item -Path "$versionInfoPath.template" -Destination $versionInfoPath -Force

  $versionInfo = Get-Content -Path $versionInfoPath -Encoding UTF8
  $versionInfo = $versionInfo -replace "{AgentMajorVersion}", $major
  $versionInfo = $versionInfo -replace "{AgentMinorVersion}", $minor
  $versionInfo = $versionInfo -replace "{AgentPatchVersion}", $patch
  $versionInfo = $versionInfo -replace "{AgentBuildVersion}", $build
  $versionInfo = $versionInfo -replace "{Year}", $buildYear

  Set-Content -Path $versionInfoPath -Value $versionInfo
}

GenerateVersionInfoFile("newrelic-infra")
GenerateVersionInfoFile("newrelic-infra-ctl")
GenerateVersionInfoFile("newrelic-infra-service")
exit 0