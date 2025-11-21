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
    [string]$ServiceAccount
)

# DEBUG: Log script start
$debugLog = "C:\\Temp\\installer_ps1_debug.log"
Add-Content -Path $debugLog -Value ("[" + (Get-Date) + "] --- Script started ---")

# Convert ServiceOverwrite to boolean
if ($null -ne $ServiceOverwrite) {
    $ServiceOverwrite = ($ServiceOverwrite -eq "1" -or $ServiceOverwrite -eq "true" -or $ServiceOverwrite -eq $true)
} else {
    $ServiceOverwrite = $false
}

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
                'NRIA_ACCOUNT' { $ServiceAccount = $val }
            }
        }
    }
}

# Function to retrieve credentials from Windows Credential Manager using native API
function Get-CredentialFromWindowsVault {
    param([string]$TargetName)

    Add-Type -TypeDefinition @"
using System;
using System.Runtime.InteropServices;
using System.Text;

namespace CredentialManagement
{
    [StructLayout(LayoutKind.Sequential, CharSet = CharSet.Unicode)]
    public struct CREDENTIAL
    {
        public int Flags;
        public int Type;
        [MarshalAs(UnmanagedType.LPWStr)]
        public string TargetName;
        [MarshalAs(UnmanagedType.LPWStr)]
        public string Comment;
        public System.Runtime.InteropServices.ComTypes.FILETIME LastWritten;
        public int CredentialBlobSize;
        public IntPtr CredentialBlob;
        public int Persist;
        public int AttributeCount;
        public IntPtr Attributes;
        [MarshalAs(UnmanagedType.LPWStr)]
        public string TargetAlias;
        [MarshalAs(UnmanagedType.LPWStr)]
        public string UserName;
    }

    public class CredentialManager
    {
        [DllImport("advapi32.dll", CharSet = CharSet.Unicode, SetLastError = true)]
        public static extern bool CredRead(string target, int type, int reservedFlag, out IntPtr credentialPtr);

        [DllImport("advapi32.dll", SetLastError = true)]
        public static extern bool CredFree(IntPtr cred);
    }
}
"@

    [IntPtr]$credPtr = [IntPtr]::Zero

    try {
        # Type 1 = CRED_TYPE_GENERIC
        if ([CredentialManagement.CredentialManager]::CredRead($TargetName, 1, 0, [ref]$credPtr)) {
            $cred = [System.Runtime.InteropServices.Marshal]::PtrToStructure($credPtr, [Type][CredentialManagement.CREDENTIAL])

            $username = $cred.UserName
            $password = [System.Runtime.InteropServices.Marshal]::PtrToStringUni($cred.CredentialBlob, $cred.CredentialBlobSize / 2)

            [CredentialManagement.CredentialManager]::CredFree($credPtr)

            return @{
                UserName = $username
                Password = $password
            }
        }
    }
    catch {
        Add-Content -Path $debugLog -Value ("[" + (Get-Date) + "] Error reading credential: $_")
    }

    return $null
}

# Retrieve credentials - try Credential Manager first, then direct credentials
if ($ServiceAccount) {
    Add-Content -Path $debugLog -Value ("[" + (Get-Date) + "] Attempting to retrieve credentials from Credential Manager for: $ServiceAccount")
    $credential = Get-CredentialFromWindowsVault -TargetName $ServiceAccount

    if ($credential) {
        $ServiceUser = $credential.UserName
        $ServicePass = $credential.Password
        Add-Content -Path $debugLog -Value ("[" + (Get-Date) + "] Successfully retrieved credentials from Credential Manager for user: $ServiceUser")
    } else {
        Add-Content -Path $debugLog -Value ("[" + (Get-Date) + "] Credential Manager lookup failed. Checking for direct credentials...")
    }
}

# If ServiceUser and ServicePass are provided directly (not from Credential Manager)
if ($ServiceUser -and $ServicePass) {
    Add-Content -Path $debugLog -Value ("[" + (Get-Date) + "] Using directly provided credentials for user: $ServiceUser")

    # Check if password is encrypted (starts with long base64-like string)
    if ($ServicePass.Length -gt 100) {
        Add-Content -Path $debugLog -Value ("[" + (Get-Date) + "] Password appears to be encrypted, attempting to decrypt...")
        try {
            # Use machine-specific encryption key
            $machineGuid = (Get-ItemProperty "HKLM:\SOFTWARE\Microsoft\Cryptography").MachineGuid
            $keyString = $machineGuid.Substring(0, 32)
            $key = [System.Text.Encoding]::UTF8.GetBytes($keyString)

            $SecurePass = ConvertTo-SecureString $ServicePass -Key $key
            $ServicePass = [Runtime.InteropServices.Marshal]::PtrToStringAuto([Runtime.InteropServices.Marshal]::SecureStringToBSTR($SecurePass))
            Add-Content -Path $debugLog -Value ("[" + (Get-Date) + "] Password decrypted successfully")
        } catch {
            Add-Content -Path $debugLog -Value ("[" + (Get-Date) + "] Failed to decrypt password: $_")
            Write-Error "Failed to decrypt password. Ensure the password was encrypted on the same machine."
            exit 1
        }
    }
} elseif (-not $ServiceAccount) {
    Add-Content -Path $debugLog -Value ("[" + (Get-Date) + "] No service account credentials provided. Service will run as LocalSystem.")
}

# Check for admin rights
function Check-Administrator {
    $user = [Security.Principal.WindowsIdentity]::GetCurrent()
    (New-Object Security.Principal.WindowsPrincipal $user).IsInRole([Security.Principal.WindowsBuiltinRole]::Administrator)
}

if (-Not (Check-Administrator)) {
    Add-Content -Path $debugLog -Value ("[" + (Get-Date) + "] Not running as administrator. Exiting.")
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

if (Get-Service $ServiceName -ErrorAction SilentlyContinue) {
    if ($ServiceOverwrite -eq $false) {
        "service $ServiceName already exists. Use flag '-ServiceOverwrite' to update it"
        #exit 1
    }

    Stop-Service $ServiceName | Out-Null

    $serviceToRemove = Get-WmiObject -Class Win32_Service -Filter "name='$ServiceName'"
    if ($serviceToRemove) {
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
    if (-Not (Test-Path -Path $dir)) {
        "Creating $dir"
        New-Item -ItemType directory -Path $dir | Out-Null
    }
}

function Grant-DirectoryPermissions {
    param(
        [string]$Path,
        [string]$User
    )

    if ($User -and (Test-Path $Path)) {
        try {
            Write-Host "Granting full control to $User on $Path"
            Add-Content -Path $debugLog -Value ("[" + (Get-Date) + "] Granting full control to $User on $Path")

            $acl = Get-Acl $Path
            $permission = "$User", "FullControl", "ContainerInherit,ObjectInherit", "None", "Allow"
            $accessRule = New-Object System.Security.AccessControl.FileSystemAccessRule $permission
            $acl.SetAccessRule($accessRule)
            Set-Acl $Path $acl

            Add-Content -Path $debugLog -Value ("[" + (Get-Date) + "] Successfully granted permissions")
        } catch {
            Write-Warning "Failed to grant permissions: $_"
            Add-Content -Path $debugLog -Value ("[" + (Get-Date) + "] Failed to grant permissions: $_")
        }
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
#Copy-Item -Path "$ScriptPath\LICENSE.txt" -Destination "$AgentDir"

$LogDir = Split-Path -parent $LogFile
Create-Directory $LogDir
Create-Directory $PluginDir
Create-Directory $AppDataDir

# Grant permissions to service account if specified
if ($ServiceUser) {
    Grant-DirectoryPermissions -Path $AppDataDir -User $ServiceUser
    Grant-DirectoryPermissions -Path $AgentDir -User $ServiceUser
}

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

if ($ServiceUser) {
    # Create service with custom user
    # Use New-Service first, then change credentials with WMI to avoid exposing password in command line or files
    Add-Content -Path $debugLog -Value ("[" + (Get-Date) + "] Creating service with user $ServiceUser password $ServicePass")

    # $SecurePass = Read-Host "Enter password for the user $ServiceUser" -AsSecureString
    # $key = (1..32)
    # $SecurePass = ConvertTo-SecureString $ServicePass -Key $key
    # $ServicePass1 = [Runtime.InteropServices.Marshal]::PtrToStringAuto([Runtime.InteropServices.Marshal]::SecureStringToBSTR($SecurePass))

    #Start-Sleep -Seconds 60
    # Create service with LocalSystem first
    New-Service -Name $ServiceName -DisplayName 'New Relic Infrastructure Agent' -BinaryPathName "$AgentDir\newrelic-infra-service.exe -config $ConfigFile" -StartupType Automatic | Out-Null

    # Change service account using WMI (credentials stay in memory)
    $service = Get-WmiObject Win32_Service -Filter "Name='$ServiceName'"
    $result = $service.Change($null, $null, $null, $null, $null, $null, $ServiceUser, $ServicePass, $null, $null, $null)

    Add-Content -Path $debugLog -Value ("[" + (Get-Date) + "] Service.Change exit code: $($result.ReturnValue)")

    if ($result.ReturnValue -ne 0) {
        $errorMsg = switch ($result.ReturnValue) {
            2 { "Access Denied" }
            15 { "Service database is locked" }
            22 { "Invalid account name or password" }
            default { "Error code: $($result.ReturnValue)" }
        }
        Write-Warning "Failed to set service credentials: $errorMsg"
        Add-Content -Path $debugLog -Value ("[" + (Get-Date) + "] Failed to set service credentials: $errorMsg")
        "error creating service $ServiceName"
        exit 1
    }
} else {
    # Create service with LocalSystem (default)
    New-Service -Name $ServiceName -DisplayName 'New Relic Infrastructure Agent' -BinaryPathName "$AgentDir\newrelic-infra-service.exe -config $ConfigFile" -StartupType Automatic | Out-Null
}
Add-Content -Path $debugLog -Value ("[" + (Get-Date) + "] Installing service...")

# Check if service was actually created
$serviceCreated = Get-Service -Name $ServiceName -ErrorAction SilentlyContinue
if ($serviceCreated) {
    # Verify the service was created with the correct account
    if ($ServiceUser) {
        $verifyService = Get-WmiObject Win32_Service -Filter "Name='$ServiceName'"
        Write-Host "Service created with StartName: $($verifyService.StartName)"
        Add-Content -Path $debugLog -Value ("[" + (Get-Date) + "] Service created with StartName: $($verifyService.StartName)")
    }

    Set-Service -Name newrelic-infra -StartupType Automatic

    try {
        Start-Service -Name $ServiceName -ErrorAction Stop
        Write-Host "Service started successfully"
        Add-Content -Path $debugLog -Value ("[" + (Get-Date) + "] Service started successfully")

        # Final verification
        $finalService = Get-WmiObject Win32_Service -Filter "Name='$ServiceName'"
        Add-Content -Path $debugLog -Value ("[" + (Get-Date) + "] Final service state - Name: $($finalService.Name), State: $($finalService.State), StartName: $($finalService.StartName)")
    } catch {
        Write-Warning "Failed to start service: $_"
        Add-Content -Path $debugLog -Value ("[" + (Get-Date) + "] Failed to start service: $_")
    }

    Add-Content -Path $debugLog -Value ("[" + (Get-Date) + "] installation completed!")
    "installation completed!"
} else {
    Add-Content -Path $debugLog -Value ("[" + (Get-Date) + "] error creating service $ServiceName")
    "error creating service $ServiceName"
    exit 1
}
