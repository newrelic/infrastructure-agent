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
    [switch]$ServiceOverwrite = $env:NRIA_OVERWRITE,
    [string]$ServiceUser      = $env:NRIA_USER,
    [string]$ServicePass      = $env:NRIA_PASS
)

# Start logging all output to a log file
#try {
#    $logPath = "C:\Temp\installer_ps1.log"
#    if (-not (Test-Path -Path (Split-Path $logPath))) {
#        New-Item -ItemType Directory -Path (Split-Path $logPath) | Out-Null
#    }
#    Start-Transcript -Path $logPath -Append
#} catch {
#    Write-Warning "Could not start transcript logging: $_"
#}

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

# Check required user rights for the current user
function Check-UserRight {
    param(
        [string]$RightName,
        [string]$UserName
    )
    $sid = (New-Object System.Security.Principal.NTAccount($UserName)).Translate([System.Security.Principal.SecurityIdentifier]).Value
    secedit /export /cfg $env:TEMP\secpol.cfg | Out-Null
    $lines = Get-Content "$env:TEMP\secpol.cfg"
    $match = $lines | Where-Object { $_ -match "^$RightName\s*=" }
    if ($match) {
        # Extract the value part after '=' and split by comma
        $value = $match -replace ".*=", ""
        $entries = $value -split ","
        # Trim entries and check for both SID and username
        foreach ($entry in $entries) {
            $trimmed = $entry.Trim().TrimStart('*')
            if ($trimmed -ieq $sid -or $trimmed -ieq $UserName) {
                return $true
            }
        }
    }
    return $false
}

$requiredRights = @{
    "SeServiceLogonRight"    = "Log on as a service"
    "SeDebugPrivilege"       = "Debug programs"
    "SeBackupPrivilege"      = "Back up files and directories"
    "SeRestorePrivilege"     = "Restore files and directories"
}

if ($ServiceUser) {
    foreach ($right in $requiredRights.Keys) {
        if (-not (Check-UserRight $right $ServiceUser)) {
            Write-Warning "$ServiceUser does NOT have '$($requiredRights[$right])' ($right)"
            $missingRight = $true
        } else {
            Write-Host "$ServiceUser has '$($requiredRights[$right])' ($right)"
        }
    }

    # Exit if any required right is missing
    if ($missingRight) {
        Write-Error "User $ServiceUser is missing one or more required rights. Exiting installation."
        exit 1
    }
} else {
    Write-Host "No ServiceUser specified, skipping privilege checks."
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
    $service = Get-WmiObject -Class Win32_Service -Filter "Name='newrelic-infra'"
    if ($ServiceUser -and $ServicePass) {
        Write-Host "Changing service logon to user $ServiceUser"
        $service.Change($null,$null,$null,$null,$null,$null,$ServiceUser,$ServicePass)
    }
    Set-Service -Name newrelic-infra -StartupType Automatic
    Start-Service -Name $ServiceName | Out-Null
    "installation completed!"
} else {
    "error creating service $ServiceName"
    exit 1
}

# Stop logging
#try {
#    Stop-Transcript
#} catch {
#    Write-Warning "Could not stop transcript logging: $_"
#}
