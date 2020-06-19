<#
.SYNOPSIS
    Installation script for the Agent Zip package.

.DESCRIPTION
    This script allows you to install the Agent automatically using custom parameters,
    which can be set as environment variables or command-line flags,
    with the latter having more precedence. If neither of both options is specified,
    then the default values will be used.
#>
param (
    [string]$LicenseKey       = $env:NRIA_LICENSE_KEY,
    [string]$AgentDir         = $env:NRIA_AGENT_DIR,
    [string]$LogFile          = $env:NRIA_LOG_FILE,
    [string]$PluginDir        = $env:NRIA_PLUGIN_DIR,
    [string]$ConfigFile       = $env:NRIA_CONFIG_FILE,
    [string]$AppDataDir       = $env:NRIA_APP_DATA_DIR,
    [string]$ServiceName      = $env:NRIA_SERVICE_NAME,
    [switch]$ServiceOverwrite = $env:NRIA_OVERWRITE
)

function Check-Administrator
{
    $user = [Security.Principal.WindowsIdentity]::GetCurrent()
    (New-Object Security.Principal.WindowsPrincipal $user).IsInRole([Security.Principal.WindowsBuiltinRole]::Administrator)
}

if (-Not (Check-Administrator))
{
    Write-Error "Admin permission is required. Please, open a Windows PowerShell session with administrative rights.";
    exit 1;
}

# The priority is:
# 1 Command-line parameter
# 2 Environment variable
# 3 Default value
if (-Not $LicenseKey)  { echo "no license key provided"; exit -1}
if (-Not $AgentDir)    { $AgentDir    =[IO.Path]::Combine($env:ProgramFiles, 'New Relic\newrelic-infra') }
if (-Not $LogFile)     { $LogFile     =[IO.Path]::Combine($AgentDir,'newrelic-infra.log') }
if (-Not $PluginDir)   { $PluginDir   =[IO.Path]::Combine($AgentDir,'integrations.d') }
if (-Not $ConfigFile)  { $ConfigFile  =[IO.Path]::Combine($AgentDir,'newrelic-infra.yml') }
if (-Not $AppDataDir)  { $AppDataDir  =[IO.Path]::Combine($env:ProgramData, 'New Relic\newrelic-infra') }
if (-Not $ServiceName) { $ServiceName ='newrelic-infra' }

if (Get-Service $ServiceName -ErrorAction SilentlyContinue)
{
    if ($ServiceOverwrite -eq $false)
    {
        "service $ServiceName already exists. Use flag '-ServiceOverwrite' to update it"
        exit 1
    }

    Stop-Service $ServiceName | Out-Null

    $serviceToRemove = Get-WmiObject -Class Win32_Service -Filter "name='$ServiceName'"
    if ($serviceToRemove)
    {
        $serviceToRemove.delete() | Out-Null
    }
}

"Installing $(Invoke-Expression '& `".\newrelic-infra.exe`" -version')"
Write-Host -NoNewline "Using the following configuration..."
[PSCustomObject] @{
    AgentDir         = $AgentDir
    LogFile          = $LogFile
    PluginDir        = $PluginDir
    ConfigFile       = $ConfigFile
    AppDataDir       = $AppDataDir
    ServiceName      = $ServiceName
    ServiceOverwrite = $ServiceOverwrite
} | Format-List

Function Create-Directory ($dir) {
    if (-Not (Test-Path -Path $dir))
    {
        "Creating $dir"
        New-Item -ItemType directory -Path $dir | Out-Null
    }
}

function Get-ScriptDirectory {
    Split-Path -parent $PSCommandPath
}

"Creating directories..."
$ScriptPath = Get-ScriptDirectory

Create-Directory $AgentDir
Create-Directory $AgentDir\custom-integrations
Create-Directory $AgentDir\newrelic-integrations
Create-Directory $AgentDir\integrations.d
Copy-Item -Path "$ScriptPath\LICENSE.txt" -Destination "$AgentDir"

$LogDir = Split-Path -parent $LogFile
Create-Directory $LogDir
Create-Directory $PluginDir
Create-Directory $AppDataDir

"Copying executables to $AgentDir..."
Copy-Item -Path "$ScriptPath\*.exe" -Destination "$AgentDir"

"Creating config file in $ConfigFile"
Clear-Content -Path $ConfigFile -ErrorAction SilentlyContinue
Add-Content -Path $ConfigFile -Value `
    "license_key: $LicenseKey",
    "log_file: $LogFile",
    "plugin_dir: $PluginDir",
    "app_data_dir: $AppDataDir"

"Installing service..."
New-Service -Name $ServiceName -DisplayName 'New Relic Infrastructure Agent' -BinaryPathName "$AgentDir\newrelic-infra-service.exe -config $ConfigFile" -StartupType Automatic | Out-Null
if ($?)
{
    Start-Service -Name $ServiceName | Out-Null
    "installation completed!"
} else {
    "error creating service $ServiceName"
    exit 1
}
