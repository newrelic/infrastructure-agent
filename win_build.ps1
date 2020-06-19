<#
    .SYNOPSIS
        This script tests builds the New Relic Infrastructure Agent
#>
param (
    # Target architecture: amd64 (default) or 386
    [ValidateSet("amd64", "386")]
    [string]$arch="amd64"
)

echo "--- Checking dependencies"

echo "Checking Go..."
go version
if (-not $?)
{
    echo "Can't find Go"
    exit -1
}

if (-Not $skipTests) {
    echo "--- Running tests"

    go test .\pkg\... .\cmd\... .\internal\...
    if (-not $?)
    {
        echo "Failed running tests"
        exit -1
    }
}

echo "--- Running Build"
$goFiles = go list .\cmd\...
go build -v $goFiles
if (-not $?)
{
    echo "Failed building files"
    exit -1
}

$goMains = @(
    ".\cmd\newrelic-infra"
    ".\cmd\newrelic-infra-ctl"
    ".\cmd\newrelic-infra-service"
)


echo "--- Running Full Build"

Foreach ($pkg in $goMains)
{
    $fileName = ([io.fileinfo]$pkg).BaseName
    echo "creating $fileName"
    $exe = ".\target\bin\windows_$arch\$fileName.exe"
    go build -o $exe $pkg
}
