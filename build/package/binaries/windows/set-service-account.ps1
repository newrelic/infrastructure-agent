<#
.SYNOPSIS
    Set the logon user account for the New Relic Infrastructure Agent service.

.DESCRIPTION
    This script sets the service account that runs the New Relic Infrastructure Agent.
    Supports regular user accounts and Group Managed Service Accounts (gMSA).
    For regular accounts, validates and grants required permissions and privileges automatically.

.PARAMETER ServiceName
    The name of the service to modify. Default: newrelic-infra

.PARAMETER CMTargetname
    Optional. The targetname in Windows Credential Manager where credentials are stored.
    Ignored if GMSAUsername is specified.

.PARAMETER GMSAUsername
    Optional. The GMSAUsername to use for the gMSA service account name ending with $.
    Must end with $ (e.g., "DOMAIN\NewRelicAgentSvc$")
    Ignored if CMTargetname is specified.

.EXAMPLE
    .\set-service-account.ps1
    Prompts for regular user account credentials interactively.

.EXAMPLE
    .\set-service-account.ps1 -CMTargetname "NewRelicServiceAccount"
    Retrieves credentials from Credential Manager.

.EXAMPLE
    .\set-service-account.ps1 -GMSAUsername "DOMAIN\NewRelicAgentSvc$"
    Uses a Group Managed Service Account (no password required).
#>

param (
    [string]$ServiceName = "newrelic-infra",
    [string]$CMTargetname,
    [string]$GMSAUsername
)

    # Check to ensure only one of $CMTargetname or $GMSAUsername is provided
if ($CMTargetname -and $GMSAUsername) {
    Write-Error "Please specify only one of -CMTargetname or -GMSAUsername, not both."
    exit 1
}

# Check for admin rights
function Test-Administrator {
    $user = [Security.Principal.WindowsIdentity]::GetCurrent()
    $principal = New-Object Security.Principal.WindowsPrincipal $user
    return $principal.IsInRole([Security.Principal.WindowsBuiltinRole]::Administrator)
}

if (-not (Test-Administrator)) {
    Write-Error "This script requires administrative privileges. Please run as Administrator."
    exit 1
}

Write-Host "=== New Relic Infrastructure Agent - Service User Change Utility ===" -ForegroundColor Cyan
Write-Host ""

# Check if service exists
$service = Get-Service -Name $ServiceName -ErrorAction SilentlyContinue
if (-not $service) {
    Write-Error "Service '$ServiceName' not found."
    exit 1
}

Write-Host "Service found: $ServiceName" -ForegroundColor Green
$serviceWMI = Get-WmiObject -Class Win32_Service -Filter "Name='$ServiceName'"
Write-Host "Current service account: $($serviceWMI.StartName)" -ForegroundColor Yellow
Write-Host ""

# Determine account type and get credentials
$credential = $null
$password = $null
$isGMSA = $false

# Check if using gMSA
if ($GMSAUsername) {
    
    if ($GMSAUsername -notlike "*$") {
        Write-Error "gMSA account name must end with $ (e.g., 'DOMAIN\NewRelicAgentSvc$')"
        exit 1
    }
    
    Write-Host "Using Group Managed Service Account (gMSA): $GMSAUsername" -ForegroundColor Cyan
    $isGMSA = $true
    $Username = $GMSAUsername
    $password = "" # gMSA doesn't need password
}
# Regular user account - Credential Manager
elseif ($CMTargetname) {
    Write-Host "Attempting to retrieve credentials from Credential Manager..." -ForegroundColor Cyan
    try {
        # Define credential type if not already defined
        if (-not ([System.Management.Automation.PSTypeName]'CredMan.CredentialManager').Type) {
            $sig = @"
[StructLayout(LayoutKind.Sequential, CharSet = CharSet.Unicode)]
public struct CREDENTIAL {
    public int Flags;
    public int Type;
    public string TargetName;
    public string Comment;
    public System.Runtime.InteropServices.ComTypes.FILETIME LastWritten;
    public int CredentialBlobSize;
    public IntPtr CredentialBlob;
    public int Persist;
    public int AttributeCount;
    public IntPtr Attributes;
    public string TargetAlias;
    public string UserName;
}

[DllImport("Advapi32.dll", EntryPoint = "CredReadW", CharSet = CharSet.Unicode, SetLastError = true)]
public static extern bool CredRead(string target, int type, int reservedFlag, out IntPtr credentialPtr);

[DllImport("Advapi32.dll", EntryPoint = "CredFree", SetLastError = true)]
public static extern void CredFree(IntPtr credentialPtr);
"@
            Add-Type -MemberDefinition $sig -Name "CredentialManager" -Namespace "CredMan"
        }
        
        $credType = [CredMan.CredentialManager]
        
        [IntPtr]$credPtr = [IntPtr]::Zero
        $success = $credType::CredRead($CMTargetname, 1, 0, [ref]$credPtr)
        
        if ($success) {
            $cred = [System.Runtime.InteropServices.Marshal]::PtrToStructure($credPtr, [type][CredMan.CredentialManager+CREDENTIAL])
            
            $Username = $cred.UserName
            Write-Host "Credentials found in Credential Manager for: $CMTargetname" -ForegroundColor Green
            Write-Host "Username: $Username" -ForegroundColor Green
            
            if ($cred.CredentialBlobSize -gt 0) {
                $password = [System.Runtime.InteropServices.Marshal]::PtrToStringUni($cred.CredentialBlob, $cred.CredentialBlobSize / 2)
                Write-Host "Password retrieved successfully" -ForegroundColor Green
            }
            
            $credType::CredFree($credPtr)
        } else {
            Write-Warning "Credentials not found in Credential Manager for target: $CMTargetname"
        }
    } catch {
        Write-Warning "Could not retrieve credentials from Credential Manager: $_"
    }
}
# Interactive prompt for regular accounts (fallback)
else {
    Write-Host "Please enter the credentials for the service account:" -ForegroundColor Cyan
    # $credential = Get-Credential -Message "Enter credentials for the New Relic Infrastructure Agent service"
    # if (-not $credential) {
    #     Write-Error "No credentials provided. Exiting."
    #     exit 1
    # }
    # $Username = $credential.UserName
    # $password = $credential.GetNetworkCredential().Password

    $Username = Read-Host "Username"
    $password = Read-Host -AsSecureString "Password"
    # Convert the secure string to plain text for use (if needed for the WMI call)
    $BSTR = [System.Runtime.InteropServices.Marshal]::SecureStringToBSTR($password)
    $passwordPlain = [System.Runtime.InteropServices.Marshal]::PtrToStringAuto($BSTR)
    [System.Runtime.InteropServices.Marshal]::ZeroFreeBSTR($BSTR)
    $password = $passwordPlain

}

# Validate username format for regular accounts
if (-not $isGMSA -and $Username -notlike "*\*") {
    $Username = ".\$Username"
}

# For gMSA, verify it's installed on this machine
if ($isGMSA) {
    Write-Host ""
    Write-Host "=== Validating gMSA Account ===" -ForegroundColor Cyan
    
    try {
        # Extract just the account name (remove domain prefix and $ suffix)
        $gmsaName = ($Username -replace '^.*\\', '') -replace '\$$', ''
        
        # Try to test the gMSA
        $gmsaTest = Test-ADServiceAccount -Identity $gmsaName -ErrorAction Stop
        
        if ($gmsaTest) {
            Write-Host "✓ gMSA account '$gmsaName' is installed and accessible on this machine" -ForegroundColor Green
        } else {
            Write-Error "gMSA account '$gmsaName' is not properly installed on this machine."
            Write-Host "Run: Install-ADServiceAccount -Identity $gmsaName" -ForegroundColor Yellow
            exit 1
        }
    } catch {
        Write-Error "Could not verify gMSA installation: $_"
        Write-Error "Please ensure the gMSA is properly installed and this computer is authorized."
        Write-Host "Run: Install-ADServiceAccount -Identity $gmsaName" -ForegroundColor Yellow
        exit 1
    }
    
    Write-Host ""
    Write-Host "Note: Folder permissions for gMSA must be configured separately (manual step required)" -ForegroundColor Yellow
}

# Validate user account and permissions
Write-Host ""
Write-Host "=== Validating User Account and Permissions ===" -ForegroundColor Cyan

# For regular accounts, strip domain prefix for folder permissions
# For gMSA, keep full name for user rights assignment
$usernameOnly = $Username -replace '^.*\\'
$usernameForRights = if ($isGMSA) { $Username } else { $usernameOnly }

if (-not $isGMSA) {

    # Check if user exists (local or domain) - skip for gMSA as already validated
    $userExists = Get-LocalUser -Name $usernameOnly -ErrorAction SilentlyContinue
    if (-not $userExists) {
        # Not a local user - check if it's a domain account
        Write-Host "User '$usernameOnly' not found as local user. Verifying domain account..." -ForegroundColor Yellow
        
        try {
            # Try to resolve the account to verify it exists (domain account)
            $account = New-Object System.Security.Principal.NTAccount($Username)
            $sid = $account.Translate([System.Security.Principal.SecurityIdentifier])
            
            # If we got here, account exists as domain account
            Write-Host "✓ Domain account verified: $Username" -ForegroundColor Green
        } catch {
            Write-Error "User '$Username' not found as either local or domain account."
            Write-Error "Please verify the account exists and is accessible."
            exit 1
        }
    } else {
        Write-Host "✓ Local user account exists: $usernameOnly" -ForegroundColor Green
    }

    # Folder permissions - only for regular accounts (not gMSA)
    # Required folders
    $appDataDir = "C:\ProgramData\New Relic\newrelic-infra"
    $agentDir = "C:\Program Files\New Relic\newrelic-infra"

    Write-Host ""
    Write-Host "Checking folder permissions..." -ForegroundColor Cyan

    function Test-UserFolderPermission {
        param([string]$Path, [string]$User)
        
        if (-not (Test-Path $Path)) {
            Write-Warning "Path does not exist: $Path"
            return $false
        }
        
        try {
            $acl = Get-Acl -Path $Path
            $userAccess = $acl.Access | Where-Object { 
                $_.IdentityReference -like "*$User*" -and 
                ($_.FileSystemRights -match "FullControl" -or $_.FileSystemRights -match "Modify")
            }
            return ($userAccess -ne $null)
        } catch {
            Write-Warning "Could not check permissions for $Path"
            return $false
        }
    }

    $appDataPermOk = Test-UserFolderPermission -Path $appDataDir -User $usernameOnly
    $agentDirPermOk = Test-UserFolderPermission -Path $agentDir -User $usernameOnly

    if (-not $appDataPermOk) {
        Write-Warning "User does not have Full Control on: $appDataDir"
        Write-Host "  Granting permissions..." -ForegroundColor Yellow
        try {
            icacls "$appDataDir" /grant "${usernameOnly}:(OI)(CI)F" /T /Q | Out-Null
            Write-Host "  Permissions granted on $appDataDir" -ForegroundColor Green
        } catch {
            Write-Error "Failed to grant permissions on $appDataDir"
            exit 1
        }
    } else {
        Write-Host "User has permissions on: $appDataDir" -ForegroundColor Green
    }

    if (-not $agentDirPermOk) {
        Write-Warning "User does not have Full Control on: $agentDir"
        Write-Host "  Granting permissions..." -ForegroundColor Yellow
        try {
            icacls "$agentDir" /grant "${usernameOnly}:(OI)(CI)F" /T /Q | Out-Null
            Write-Host "  Permissions granted on $agentDir" -ForegroundColor Green
        } catch {
            Write-Error "Failed to grant permissions on $agentDir"
            exit 1
        }
    } else {
        Write-Host "User has permissions on: $agentDir" -ForegroundColor Green
    }
}

# Grant user rights for both regular accounts and gMSA
Write-Host ""
Write-Host "Checking user privileges..." -ForegroundColor Cyan

function Grant-UserRight {
        param([string]$User, [string]$Right, [string]$Description)
        
        Write-Host "  Granting: $Description" -ForegroundColor Yellow
        
        try {
            $tempFile = [System.IO.Path]::GetTempFileName()
            secedit /export /cfg $tempFile /quiet | Out-Null
            
            $content = Get-Content $tempFile
            $userSid = (New-Object System.Security.Principal.NTAccount($User)).Translate([System.Security.Principal.SecurityIdentifier]).Value
            
            $newContent = @()
            foreach ($line in $content) {
                if ($line -like "$Right*") {
                    if ($line -notlike "*$userSid*") {
                        $newContent += $line.TrimEnd() + ",*$userSid"
                    } else {
                        $newContent += $line
                    }
                } else {
                    $newContent += $line
                }
            }
            
            $newContent | Set-Content $tempFile
            secedit /configure /db secedit.sdb /cfg $tempFile /quiet | Out-Null
            
            Remove-Item $tempFile -Force -ErrorAction SilentlyContinue
            Remove-Item secedit.sdb -Force -ErrorAction SilentlyContinue
            
            Write-Host "    ✓ Granted: $Description" -ForegroundColor Green
            return $true
        } catch {
            Write-Error "Failed to grant $Description : $_"
            return $false
        }
    }

$requiredRights = @{
    "SeServiceLogonRight" = "Log on as a service"
    "SeDebugPrivilege" = "Debug programs"
    "SeBackupPrivilege" = "Back up files and directories"
    "SeRestorePrivilege" = "Restore files and directories"
}

foreach ($right in $requiredRights.Keys) {
    Grant-UserRight -User $usernameForRights -Right $right -Description $requiredRights[$right] | Out-Null
}

Write-Host ""
Write-Host "=== Changing Service Account ===" -ForegroundColor Cyan

Write-Host "Stopping service..." -ForegroundColor Yellow
try {
    Stop-Service -Name $ServiceName -Force -ErrorAction Stop
    Write-Host "✓ Service stopped" -ForegroundColor Green
} catch {
    Write-Warning "Could not stop service (may already be stopped): $_"
}

Write-Host "Updating service account to: $Username" -ForegroundColor Yellow
try {
    $serviceWMI = Get-WmiObject -Class Win32_Service -Filter "Name='$ServiceName'"
    
    # For gMSA, password must be empty string
    if ($isGMSA) {
        $result = $serviceWMI.Change($null, $null, $null, $null, $null, $null, $Username, "", $null, $null, $null)
    } else {
        $result = $serviceWMI.Change($null, $null, $null, $null, $null, $null, $Username, $password, $null, $null, $null)
    }
    
    if ($result.ReturnValue -eq 0) {
        Write-Host "✓ Service account updated successfully" -ForegroundColor Green
    } else {
        Write-Error "Failed to change service account. WMI error code: $($result.ReturnValue)"
        exit 1
    }
} catch {
    Write-Error "Failed to change service account: $_"
    exit 1
}

Write-Host "Starting service..." -ForegroundColor Yellow
try {
    Start-Service -Name $ServiceName -ErrorAction Stop
    Start-Sleep -Seconds 2
    
    $serviceStatus = Get-Service -Name $ServiceName
    if ($serviceStatus.Status -eq 'Running') {
        Write-Host "Service started successfully" -ForegroundColor Green
    } else {
        Write-Warning "Service is in state: $($serviceStatus.Status)"
    }
} catch {
    Write-Error "Failed to start service: $_"
    Write-Host "The service account has been changed, but the service failed to start." -ForegroundColor Yellow
    Write-Host "Please check the Windows Event Logs for details." -ForegroundColor Yellow
    exit 1
}

Write-Host ""
Write-Host "=== Summary ===" -ForegroundColor Cyan
$finalService = Get-WmiObject -Class Win32_Service -Filter "Name='$ServiceName'"
Write-Host "Service Name:    $ServiceName"
Write-Host "Service Account: $($finalService.StartName)"
Write-Host "Service Status:  $((Get-Service -Name $ServiceName).Status)"
Write-Host ""
Write-Host "Service account change completed successfully!" -ForegroundColor Green
