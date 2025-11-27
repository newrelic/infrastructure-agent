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
    [string]$ServiceName      = $env:NRIA_SERVICE_NAME
)

# Check for admin rights
function Check-Administrator {
    $user = [Security.Principal.WindowsIdentity]::GetCurrent()
    (New-Object Security.Principal.WindowsPrincipal $user).IsInRole([Security.Principal.WindowsBuiltinRole]::Administrator)
}

if (-Not (Check-Administrator)) {
    Write-Error "Admin permission is required. Please, open a Windows PowerShell session with administrative rights."
    exit 1
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
#if (-Not $LicenseKey)  { echo "no license key provided"; exit -1}
if (-Not $AgentDir)    { $AgentDir    = [IO.Path]::Combine($env:ProgramFiles, 'New Relic\newrelic-infra') }
if (-Not $LogFile)     { $LogFile     = [IO.Path]::Combine($AgentDir,'newrelic-infra.log') }
if (-Not $PluginDir)   { $PluginDir   = [IO.Path]::Combine($AgentDir,'integrations.d') }
if (-Not $ConfigFile)  { $ConfigFile  = [IO.Path]::Combine($AgentDir,'newrelic-infra.yml') }
if (-Not $AppDataDir)  { $AppDataDir  = [IO.Path]::Combine($env:ProgramData, 'New Relic\newrelic-infra') }
if (-Not $ServiceName) { $ServiceName = 'newrelic-infra' }

# Check if service already exists
$existingService = Get-Service $ServiceName -ErrorAction SilentlyContinue
$isUpgrade = $false
$preservedAccount = $null

if ($existingService) {
    Write-Host "Service $ServiceName already exists. Performing upgrade..."

    # Get the existing service account before stopping
    $existingServiceWMI = Get-WmiObject -Class Win32_Service -Filter "name='$ServiceName'"
    if ($existingServiceWMI) {
        $preservedAccount = $existingServiceWMI.StartName
        Write-Host "Existing service runs as: $preservedAccount"
    }

    # Stop the service if running
    if ($existingService.Status -eq 'Running') {
        Write-Host "Stopping service $ServiceName..."
        Stop-Service $ServiceName -Force | Out-Null
        Start-Sleep -Seconds 2
    }

    $isUpgrade = $true
} else {
    Write-Host "Service $ServiceName does not exist. Performing fresh installation..."
}



if (Test-Path "$AgentDir\newrelic-infra.exe") {
    $versionOutput = & "$AgentDir\newrelic-infra.exe" -version
    "Installing $versionOutput"
}
Write-Host -NoNewline "Using the following configuration..."
[PSCustomObject] @{
    AgentDir         = $AgentDir
    LogFile          = $LogFile
    PluginDir        = $PluginDir
    ConfigFile       = $ConfigFile
    AppDataDir       = $AppDataDir
    ServiceName      = $ServiceName
} | Format-List

Function Create-Directory ($dir) {
    if (-Not (Test-Path -Path $dir)) {
        "Creating $dir"
        New-Item -ItemType directory -Path $dir | Out-Null
    }
}

function Get-ScriptDirectory {
    Split-Path -parent $PSCommandPath
}

$ScriptPath = Get-ScriptDirectory

# Create directories only for fresh installation
if (-not $isUpgrade) {
    "Creating directories..."
    Create-Directory $AgentDir
    Create-Directory $AgentDir\custom-integrations
    Create-Directory $AgentDir\newrelic-integrations
    Create-Directory $AgentDir\integrations.d
    Copy-Item -Path "$ScriptPath\LICENSE.txt" -Destination "$AgentDir"

    $LogDir = Split-Path -parent $LogFile
    Create-Directory $LogDir
    Create-Directory $PluginDir
    Create-Directory $AppDataDir
} else {
    Write-Host "Upgrade detected - skipping directory creation"
}

"Copying executables to $AgentDir..."
Copy-Item -Path "$ScriptPath\*.exe" -Destination "$AgentDir" -Force

# For upgrades, only update config if it doesn't exist
if ($isUpgrade -and (Test-Path $ConfigFile)) {
    Write-Host "Preserving existing configuration file: $ConfigFile"
} else {
    "Creating config file in $ConfigFile"
    Clear-Content -Path $ConfigFile -ErrorAction SilentlyContinue
    Add-Content -Path $ConfigFile -Value `
        "license_key: $LicenseKey",
        "log_file: $LogFile",
        "plugin_dir: $PluginDir",
        "app_data_dir: $AppDataDir"
}

if ($isUpgrade) {
    # Upgrade scenario: restart service with preserved account
    Write-Host "Restarting service with preserved account: $preservedAccount"
    
    # Grant permissions if using a custom service account (not LocalSystem)
    if ($preservedAccount -and $preservedAccount -ne "LocalSystem") {
        Write-Host "Granting permissions to $preservedAccount on data directories..."
        $username = $preservedAccount -replace '^\.\\'  # Remove .\ prefix if present
        icacls "$AppDataDir" /grant "${username}:(OI)(CI)F" /T /Q | Out-Null
    }
    
    try {
        Start-Service -Name $ServiceName -ErrorAction Stop
        "Upgrade completed successfully!"
    } catch {
        Write-Warning "Failed to restart service: $_"
        exit 1
    }
} else {
    # Fresh installation scenario: create service with LocalSystem
    New-Service -Name $ServiceName -DisplayName 'New Relic Infrastructure Agent' -BinaryPathName "$AgentDir\newrelic-infra-service.exe -config $ConfigFile" -StartupType Automatic | Out-Null

    $serviceCreated = Get-Service -Name $ServiceName -ErrorAction SilentlyContinue
    if ($serviceCreated) {
        try {
            Start-Service -Name $ServiceName -ErrorAction Stop
            "Installation completed!"
        } catch {
            Write-Warning "Failed to start service: $_"
        }
    } else {
        "error creating service $ServiceName"
        exit 1
    }
}
