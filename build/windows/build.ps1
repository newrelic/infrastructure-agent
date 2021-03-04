<#
    .SYNOPSIS
        This script build the binaries of New Relic Infrastructure Agent
#>
param (
    # Target architecture: amd64 (default) or 386
    [ValidateSet("amd64", "386")]
    [string]$arch="amd64",

    [string]$version="0.0.0",
    [string]$commit="default",

    # Skip signing
    [switch]$skipSigning=$false,
    # Signing tool
    [string]$signtool='"C:\Program Files (x86)\Windows Kits\10\bin\x64\signtool.exe"'
)
$scriptPath = split-path -parent $MyInvocation.MyCommand.Definition
$workspace = "$scriptPath\..\.."

# Source build Functions.
. $scriptPath/scripts/functions.ps1

Write-Output "--- Cleaning target..."
Remove-Item -Path "target" -Force -Recurse -ErrorAction Ignore

Write-Output "--- Checking dependencies"
Write-Output "Checking Go..."
go version
if (-not $?)
{
    Write-Output "Can't find Go"
    exit -1
}

Write-Output "Downloading go modules..."
go mod download

Write-Output "Installing goversioninfo..."
$Env:Path+= ";" + $Env:GOPATH + "\bin"
go get github.com/josephspurrier/goversioninfo/cmd/goversioninfo

$goMains = @(
    "$workspace\cmd\newrelic-infra"
    "$workspace\cmd\newrelic-infra-ctl"
    "$workspace\cmd\newrelic-infra-service"
    "$workspace\tools\yamlgen"
)

Write-Output "--- Generating code..."
Invoke-expression -Command "$scriptPath\scripts\set_exe_metadata.ps1 -version ${version}"
if ($lastExitCode -ne 0) {

    Write-Output "Failed to generate code"
    exit -1
}

Foreach ($pkg in $goMains)
{
    Write-Output "generating $pkg"
    go generate $pkg
    if (-not $?)
    {
        Write-Output "Failed generate code $pkg"
        exit -1
    }
}

Write-Output "--- Running Build"
$env:GOOS="windows"
$env:GOARCH=$arch

$goFiles = go list $workspace\cmd\...
go build -v $goFiles
if (-not $?)
{
    Write-Output "Failed building files"
    exit -1
}

Write-Output "--- Running Full Build"

Foreach ($pkg in $goMains)
{
    $fileName = ([io.fileinfo]$pkg).BaseName
    Write-Output "creating $fileName"

    $exe = "$workspace\target\bin\windows_$arch\$fileName.exe"
    go build -ldflags "-X 'main.buildVersion=$version' -X 'main.gitCommit=$commit'" -o $exe $pkg
    if (-Not $skipSigning) {
        SignExecutable -executable "$exe"
    }
}
