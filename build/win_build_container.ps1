<#
    .SYNOPSIS
        This script tests builds the New Relic Infrastructure Agent Windows container image
#>
param (
    # Target base image:
    [ValidateSet("1809", "1909", "2004" )]
    [string]$baseImageTag="1809",

    # Docker image namespace:
    [string]$dockerImageNamespace="newrelic",

    # Docker image repo:
    [string]$dockerImageRepo="infrastructure",

    # Agent version:
    [string]$agentVersion="0.0.0"

)

$dockerTag = "$dockerImageNamespace/$dockerImageRepo"

$agentBin = "./target/bin/windows_amd64/newrelic-infra.exe"
$agentCtlBin = "./target/bin/windows_amd64/newrelic-infra-ctl.exe"
$agentServiceBin = "./target/bin/windows_amd64/newrelic-infra-service.exe"

$projectDir = (Get-Item .).FullName
$dockerFile = (Resolve-Path .\build\container\Dockerfile.windows).Path

echo "--- Building container with image $baseImageTag, agent version at $agentVersion and Docker tag as $dockerTag"

docker build `
    --pull `
    --build-arg base_image_tag="$baseImageTag-amd64" `
    --build-arg image_version=$agentVersion `
    --build-arg agent_version=$agentVersion `
    --build-arg agent_bin=$agentBin `
    --build-arg agent_ctl_bin=$agentCtlBin `
    --build-arg agent_service_bin=$agentServiceBin `
    -f $dockerFile `
    -t $dockerTag .
