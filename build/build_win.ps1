<#
    .SYNOPSIS
        This script build the binaries and run the tests of New Relic Infrastructure Agent
#>
param (
    # Target architecture: amd64 (default) or 386
    [ValidateSet("amd64", "386")]
    [string]$arch="amd64",

    [string]$version="0.0.0",

    # Skip tests
    [switch]$skipTests=$false,

    # Skip build
    [switch]$skipBuild=$false,

    # Skip signing
    [switch]$skipSigning=$false,
    # Signing tool
    [string]$signtool='"C:\Program Files (x86)\Windows Kits\10\bin\x64\signtool.exe"'
)
$scriptPath = split-path -parent $MyInvocation.MyCommand.Definition
$workspace = "$scriptPath\.."

$env:GOOS="windows"
$env:GOARCH=$arch

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

echo $Env:Path
echo $Env:GOPATH
ls $Env:Path

ls $Env:GOBIN



if (-Not $skipTests) {
    Write-Output "--- Running tests"

    go test $workspace\pkg\... $workspace\cmd\... $workspace\internal\... $workspace\test\...
    if (-not $?)
    {
        Write-Output "Failed running tests"
        exit -1
    }
}

if ($skipBuild) {
    Write-Output "--- Build step skipped"
    exit 0
}

Write-Output "--- Cleaning target..."
Remove-Item -Path "target" -Force -Recurse -ErrorAction Ignore

$goMains = @(
    "$workspace\cmd\newrelic-infra"
    "$workspace\cmd\newrelic-infra-ctl"
    "$workspace\cmd\newrelic-infra-service"
    "$workspace\tools\yamlgen"
)

Write-Output "--- Generating code..."
Invoke-expression -Command "$scriptPath\set_exe_metadata.ps1 -version ${version}"
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
    go build -ldflags "-X main.buildVersion=$version" -o $exe $pkg
    if (-Not $skipSigning) {
        Invoke-Expression "& $signtool sign /d 'New Relic Infrastructure Agent' /n 'New Relic, Inc.' $exe"
        if ($lastExitCode -ne 0) {
            Write-Output "Failed to sign $exe"
            exit -1
        }
    }
}
