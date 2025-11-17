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
    [string]$ServiceOverwrite = $env:NRIA_OVERWRITE,
    [string]$ServiceUser,
    [string]$ServicePass
)

# DEBUG: Log script start
$debugLog = "C:\\Temp\\installer_ps1_debug.log"
Add-Content -Path $debugLog -Value ("[" + (Get-Date) + "] --- Script started ---")

# Convert ServiceOverwrite to boolean (fix misplaced parenthesis)
if ($null -ne $ServiceOverwrite) {
    $ServiceOverwrite = ($ServiceOverwrite -eq "1" -or $ServiceOverwrite -eq "true" -or $ServiceOverwrite -eq $true)
} else {
    $ServiceOverwrite = $false
}


# Parse CustomActionData if present (for MSI installs)
$CustomActionData = $env:CustomActionData
# Parse CustomActionData if present (for MSI installs)
$CustomActionData = $env:CustomActionData
Add-Content -Path $debugLog -Value ("[" + (Get-Date) + "] CustomActionData: $CustomActionData")
if ($CustomActionData) {
    $pairs = $CustomActionData -split ';'
    foreach ($pair in $pairs) {
        if ($pair -match '^(.*?)=(.*)$') {
            $key = $matches[1]
            $val = $matches[2]
            Add-Content -Path $debugLog -Value ("[" + (Get-Date) + "] Parsed: $key = $val")
            switch ($key) {
                'NRIA_USER' { $ServiceUser = $val }
                'NRIA_PASS' { $ServicePass = $val }
            # Add-Content -Path $debugLog -Value ("[" + (Get-Date) + "] ServiceUser param: $ServiceUser")
            # Add-Content -Path $debugLog -Value ("[" + (Get-Date) + "] ServicePass param: $ServicePass")

            }
        }
    }
}

 function Check-Administrator
 {
     $user = [Security.Principal.WindowsIdentity]::GetCurrent()
     (New-Object Security.Principal.WindowsPrincipal $user).IsInRole([Security.Principal.WindowsBuiltinRole]::Administrator)
 }


 if (-Not (Check-Administrator))
 {
    Add-Content -Path $debugLog -Value ("[" + (Get-Date) + "] Not running as administrator. Exiting.")
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
    # Extract postfix (username without prefix)
    $User = $UserName
    if ($UserName -match "^(.*\\)(.+)$") {
        $User = $Matches[2]
    }
    secedit /export /cfg $env:TEMP\secpol.cfg | Out-Null
    $lines = Get-Content "$env:TEMP\secpol.cfg"
    $match = $lines | Where-Object { $_ -match "^$RightName\s*=" }
    if ($match) {
        # Extract the value part after '=' and split by comma
        $value = $match -replace ".*=", ""
        $entries = $value -split ","
        # Trim entries and check for SID, full username, and postfix
        foreach ($entry in $entries) {
            $trimmed = $entry.Trim().TrimStart('*')
            if ($trimmed -ieq $sid -or $trimmed -ieq $UserName -or $trimmed -ieq $User) {
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
    Add-Content -Path $debugLog -Value ("[" + (Get-Date) + "] Checking rights for ServiceUser: $ServiceUser")
    foreach ($right in $requiredRights.Keys) {
        if (-not (Check-UserRight $right $ServiceUser)) {
            Write-Warning "$ServiceUser does NOT have '$($requiredRights[$right])' ($right)"
            Add-Content -Path $debugLog -Value ("[" + (Get-Date) + "] $ServiceUser does NOT have $($requiredRights[$right]) ($right)")
            $missingRight = $true
        } else {
            Write-Host "$ServiceUser has '$($requiredRights[$right])' ($right)"
            Add-Content -Path $debugLog -Value ("[" + (Get-Date) + "] $ServiceUser has $($requiredRights[$right]) ($right)")
        }
    }

    # Exit if any required right is missing
    if ($missingRight) {
        Add-Content -Path $debugLog -Value ("[" + (Get-Date) + "] User $ServiceUser is missing one or more required rights. Exiting installation.")
        Write-Error "User $ServiceUser is missing one or more required rights. Exiting installation."
        exit 1
    }
} else {
    Write-Host "No ServiceUser specified, skipping privilege checks."
    Add-Content -Path $debugLog -Value ("[" + (Get-Date) + "] No ServiceUser specified, skipping privilege checks.")
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



if (Test-Path "$AgentDir\newrelic-infra.exe") {
    $versionOutput = & "$AgentDir\newrelic-infra.exe" -version
    Add-Content -Path $debugLog -Value ("[" + (Get-Date) + "] Installing $versionOutput")
} else {
    Add-Content -Path $debugLog -Value ("[" + (Get-Date) + "] newrelic-infra.exe not found in $AgentDir")
}
Add-Content -Path $debugLog -Value ("[" + (Get-Date) + "] Using configuration: AgentDir=$AgentDir, LogFile=$LogFile, PluginDir=$PluginDir, ConfigFile=$ConfigFile, AppDataDir=$AppDataDir, ServiceName=$ServiceName, ServiceOverwrite=$ServiceOverwrite")
"Installing $versionOutput"
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
$debugDirs = @($AgentDir, "$AgentDir\\custom-integrations", "$AgentDir\\newrelic-integrations", "$AgentDir\\integrations.d", $LogDir, $PluginDir, $AppDataDir)
foreach ($d in $debugDirs) { Add-Content -Path $debugLog -Value ("[" + (Get-Date) + "] Creating directory: $d") }
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
Add-Content -Path $debugLog -Value ("[" + (Get-Date) + "] Copying executables to $AgentDir")
Copy-Item -Path "$ScriptPath\*.exe" -Destination "$AgentDir"

"Creating config file in $ConfigFile"
Add-Content -Path $debugLog -Value ("[" + (Get-Date) + "] Creating config file in $ConfigFile")
Clear-Content -Path $ConfigFile -ErrorAction SilentlyContinue
Add-Content -Path $ConfigFile -Value `
    "license_key: $LicenseKey",
    "log_file: $LogFile",
    "plugin_dir: $PluginDir",
    "app_data_dir: $AppDataDir"


New-Service -Name $ServiceName -DisplayName 'New Relic Infrastructure Agent' -BinaryPathName "$AgentDir\newrelic-infra-service.exe -config $ConfigFile" -StartupType Automatic | Out-Null
Add-Content -Path $debugLog -Value ("[" + (Get-Date) + "] Installing service...")
if ($?)
{
    $service = Get-WmiObject -Class Win32_Service -Filter "Name='newrelic-infra'"
    if ($ServiceUser -and $ServicePass) {
        Write-Host "Changing service logon to user $ServiceUser"
        Add-Content -Path $debugLog -Value ("[" + (Get-Date) + "] Changing service logon to user $ServiceUser")
        $service.Change($null,$null,$null,$null,$null,$null,$ServiceUser,$ServicePass)
    }
    Set-Service -Name newrelic-infra -StartupType Automatic
    Start-Service -Name $ServiceName | Out-Null
    Add-Content -Path $debugLog -Value ("[" + (Get-Date) + "] installation completed!")
    "installation completed!"
} else {
    Add-Content -Path $debugLog -Value ("[" + (Get-Date) + "] error creating service $ServiceName")
    "error creating service $ServiceName"
    exit 1
}
