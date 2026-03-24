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

.PARAMETER SkipValidation
    Optional. Skip gMSA validation check. Use when Test-ADServiceAccount is unavailable
    or when you're confident the gMSA is properly configured.

.EXAMPLE
    .\set-service-account.ps1
    Prompts for regular user account credentials interactively.

.EXAMPLE
    .\set-service-account.ps1 -CMTargetname "NewRelicServiceAccount"
    Retrieves credentials from Credential Manager.

.EXAMPLE
    .\set-service-account.ps1 -GMSAUsername "DOMAIN\NewRelicAgentSvc$"
    Uses a Group Managed Service Account (no password required).

.EXAMPLE
    .\set-service-account.ps1 -GMSAUsername "DOMAIN\NewRelicAgentSvc$" -SkipValidation
    Uses a gMSA and skips the Test-ADServiceAccount validation check.
#>

param (
    [string]$ServiceName = "newrelic-infra",
    [string]$CMTargetname,
    [string]$GMSAUsername,
    [switch]$SkipValidation
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

# Load LSA native types unconditionally at script start — required for both credential
# retrieval (Get-ServiceStoredCredential) and user rights management (Grant/Revoke-UserRights).
if (-not ([System.Management.Automation.PSTypeName]'LSAUtils.NativeMethods').Type) {
    Add-Type -TypeDefinition @"
using System;
using System.Runtime.InteropServices;
namespace LSAUtils {
    [StructLayout(LayoutKind.Sequential)]
    public struct LSA_UNICODE_STRING {
        public ushort Length;
        public ushort MaximumLength;
        public IntPtr Buffer;
    }
    [StructLayout(LayoutKind.Sequential)]
    public struct LSA_OBJECT_ATTRIBUTES {
        public int Length;
        public IntPtr RootDirectory;
        public IntPtr ObjectName;
        public uint Attributes;
        public IntPtr SecurityDescriptor;
        public IntPtr SecurityQualityOfService;
    }
    public class NativeMethods {
        [DllImport("advapi32.dll")] public static extern uint LsaOpenPolicy(ref LSA_UNICODE_STRING SystemName, ref LSA_OBJECT_ATTRIBUTES ObjectAttributes, uint DesiredAccess, out IntPtr PolicyHandle);
        [DllImport("advapi32.dll")] public static extern uint LsaRetrievePrivateData(IntPtr PolicyHandle, ref LSA_UNICODE_STRING KeyName, out IntPtr PrivateData);
        [DllImport("advapi32.dll")] public static extern uint LsaFreeMemory(IntPtr Buffer);
        [DllImport("advapi32.dll")] public static extern uint LsaClose(IntPtr ObjectHandle);
        [DllImport("advapi32.dll")] public static extern uint LsaEnumerateAccountRights(IntPtr PolicyHandle, IntPtr AccountSid, out IntPtr UserRights, out uint CountOfRights);
        [DllImport("advapi32.dll")] public static extern uint LsaAddAccountRights(IntPtr PolicyHandle, IntPtr AccountSid, LSA_UNICODE_STRING[] UserRights, uint CountOfRights);
        [DllImport("advapi32.dll")] public static extern uint LsaRemoveAccountRights(IntPtr PolicyHandle, IntPtr AccountSid, bool AllRights, LSA_UNICODE_STRING[] UserRights, uint CountOfRights);

        private static LSA_UNICODE_STRING[] BuildRightsArray(string[] rights) {
            var arr = new LSA_UNICODE_STRING[rights.Length];
            for (int i = 0; i < rights.Length; i++) {
                arr[i].Buffer = Marshal.StringToHGlobalUni(rights[i]);
                arr[i].Length = (ushort)(rights[i].Length * 2);
                arr[i].MaximumLength = (ushort)((rights[i].Length + 1) * 2);
            }
            return arr;
        }
        private static void FreeRightsArray(LSA_UNICODE_STRING[] arr) {
            foreach (var r in arr) Marshal.FreeHGlobal(r.Buffer);
        }
        public static string[] GetAccountRights(IntPtr policyHandle, IntPtr accountSid) {
            IntPtr userRights = IntPtr.Zero; uint count = 0;
            uint status = LsaEnumerateAccountRights(policyHandle, accountSid, out userRights, out count);
            if (status != 0 || userRights == IntPtr.Zero) return new string[0];
            try {
                var rights = new string[count];
                int size = Marshal.SizeOf(typeof(LSA_UNICODE_STRING));
                for (uint i = 0; i < count; i++) {
                    IntPtr item = new IntPtr(userRights.ToInt64() + i * size);
                    var s = (LSA_UNICODE_STRING)Marshal.PtrToStructure(item, typeof(LSA_UNICODE_STRING));
                    rights[i] = Marshal.PtrToStringUni(s.Buffer, s.Length / 2);
                }
                return rights;
            } finally { LsaFreeMemory(userRights); }
        }
        public static uint AddRights(IntPtr policyHandle, IntPtr accountSid, string[] rights) {
            var arr = BuildRightsArray(rights);
            try { return LsaAddAccountRights(policyHandle, accountSid, arr, (uint)rights.Length); }
            finally { FreeRightsArray(arr); }
        }
        public static uint RemoveRights(IntPtr policyHandle, IntPtr accountSid, string[] rights) {
            var arr = BuildRightsArray(rights);
            try { return LsaRemoveAccountRights(policyHandle, accountSid, false, arr, (uint)rights.Length); }
            finally { FreeRightsArray(arr); }
        }
    }
}
"@
}

# Check if service exists
$service = Get-Service -Name $ServiceName -ErrorAction SilentlyContinue
if (-not $service) {
    Write-Error "Service '$ServiceName' not found."
    exit 1
}

Write-Host "Service found: $ServiceName" -ForegroundColor Green
$serviceWMI = Get-WmiObject -Class Win32_Service -Filter "Name='$ServiceName'"
$originalServiceAccount = $serviceWMI.StartName
Write-Host "Current service account: $originalServiceAccount" -ForegroundColor Yellow

# Reads the service account password from LSA secrets (_SC_<ServiceName>) where Windows
# stores it when a custom account is configured. Returns a SecureString or $null.
function Get-ServiceStoredCredential {
    param([string]$SvcName)

    try {
        $objAttr = New-Object LSAUtils.LSA_OBJECT_ATTRIBUTES
        $objAttr.Length = [System.Runtime.InteropServices.Marshal]::SizeOf($objAttr)
        $sysName = New-Object LSAUtils.LSA_UNICODE_STRING

        [IntPtr]$policyHandle = [IntPtr]::Zero
        $status = [LSAUtils.NativeMethods]::LsaOpenPolicy([ref]$sysName, [ref]$objAttr, 0x00000004, [ref]$policyHandle)
        if ($status -ne 0) { return $null }

        try {
            $secretName = "_SC_$SvcName"
            $keyName = New-Object LSAUtils.LSA_UNICODE_STRING
            $keyName.Buffer = [System.Runtime.InteropServices.Marshal]::StringToHGlobalUni($secretName)
            $keyName.Length = [ushort]($secretName.Length * 2)
            $keyName.MaximumLength = [ushort](($secretName.Length + 1) * 2)

            [IntPtr]$privateData = [IntPtr]::Zero
            $status = [LSAUtils.NativeMethods]::LsaRetrievePrivateData($policyHandle, [ref]$keyName, [ref]$privateData)
            [System.Runtime.InteropServices.Marshal]::FreeHGlobal($keyName.Buffer)

            if ($status -ne 0 -or $privateData -eq [IntPtr]::Zero) { return $null }

            try {
                $lsaStr = [System.Runtime.InteropServices.Marshal]::PtrToStructure($privateData, [type][LSAUtils.LSA_UNICODE_STRING])
                # Read char-by-char directly into SecureString — never stored as plain text
                $securePassword = New-Object System.Security.SecureString
                for ($i = 0; $i -lt ($lsaStr.Length / 2); $i++) {
                    $securePassword.AppendChar([char][System.Runtime.InteropServices.Marshal]::ReadInt16($lsaStr.Buffer, $i * 2))
                }
                $securePassword.MakeReadOnly()
                return $securePassword
            } finally {
                [LSAUtils.NativeMethods]::LsaFreeMemory($privateData) | Out-Null
            }
        } finally {
            [LSAUtils.NativeMethods]::LsaClose($policyHandle) | Out-Null
        }
    } catch {
        return $null
    }
}

# Built-in system accounts (LocalSystem, NT AUTHORITY\*, NT SERVICE\*) don't need a
# password for WMI revert. Custom local/domain accounts do — read from LSA secrets.
$originalIsBuiltin = ($originalServiceAccount -notlike '*\*') -or ($originalServiceAccount -match '^NT ')
$originalCredential = $null

if (-not $originalIsBuiltin) {
    $storedPassword = Get-ServiceStoredCredential -SvcName $ServiceName
    if ($storedPassword) {
        $originalCredential = New-Object System.Management.Automation.PSCredential($originalServiceAccount, $storedPassword)
        Write-Host "✓ Original account credentials captured for rollback" -ForegroundColor Green
    } else {
        # LSA secret not available — prompt once upfront so rollback can restore the service
        # account automatically if something goes wrong later.
        Write-Warning "Could not retrieve stored credentials for '$originalServiceAccount' from LSA secrets."
        Write-Host "Enter the password for '$originalServiceAccount' (used only if rollback is needed):" -ForegroundColor Yellow
        $rollbackPassword = Read-Host -AsSecureString "Rollback password for $originalServiceAccount"
        $originalCredential = New-Object System.Management.Automation.PSCredential($originalServiceAccount, $rollbackPassword)
        Write-Host "✓ Original account credentials captured for rollback" -ForegroundColor Green
    }
}

Write-Host ""

# Determine account type and get credentials
$credential = $null
$isGMSA = $false
$isBuiltinTarget = $false  # set here so all branches have it defined; updated per-branch below

# Check if using gMSA
if ($GMSAUsername) {
    
    if ($GMSAUsername -notlike "*$") {
        Write-Error "gMSA account name must end with $ (e.g., 'DOMAIN\NewRelicAgentSvc$')"
        exit 1
    }
    
    Write-Host "Using Group Managed Service Account (gMSA): $GMSAUsername" -ForegroundColor Cyan
    $isGMSA = $true
    $Username = $GMSAUsername
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

            # Compute $isBuiltinTarget now that $Username is known for this branch
            $isBuiltinTarget = ($Username -notlike '*\*') -or ($Username -match '^NT (AUTHORITY|SERVICE)\\')

            if ($cred.CredentialBlobSize -gt 0) {
                $securePassword = New-Object System.Security.SecureString
                $charCount = $cred.CredentialBlobSize / 2
                for ($i = 0; $i -lt $charCount; $i++) {
                    $charValue = [System.Runtime.InteropServices.Marshal]::ReadInt16($cred.CredentialBlob, $i * 2)
                    $securePassword.AppendChar([char]$charValue)
                }
                $securePassword.MakeReadOnly()
                $credential = New-Object System.Management.Automation.PSCredential($Username, $securePassword)
                Write-Host "Password retrieved successfully" -ForegroundColor Green
            } elseif (-not $isBuiltinTarget) {
                # Entry exists but has no password blob and account is not a built-in — cannot proceed
                $credType::CredFree($credPtr)
                Write-Error "Credential Manager entry '$CMTargetname' exists but has no stored password (CredentialBlobSize is 0). Update the credential entry and retry."
                exit 1
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
    $Username = Read-Host "Username"

    # Built-in accounts (LocalSystem, NT AUTHORITY\*, NT SERVICE\*) don't have a password
    $isBuiltinTarget = ($Username -notlike '*\*') -or ($Username -match '^NT (AUTHORITY|SERVICE)\\')
    if (-not $isBuiltinTarget) {
        $securePassword = Read-Host -AsSecureString "Password"
        $credential = New-Object System.Management.Automation.PSCredential($Username, $securePassword)
    }
}

# Validate username format for regular accounts
if (-not $isGMSA -and $Username -notlike "*\*") {
    $Username = ".\$Username"
}

# Normalize both for comparison: treat LocalSystem / .\LocalSystem / HOSTNAME\LocalSystem as equal
$normalizedNew = $Username -replace "^\.\\"
$normalizedOriginal = $originalServiceAccount -replace "^\.\\" -replace "^$env:COMPUTERNAME\\", ""
if ($normalizedNew -ieq $normalizedOriginal) {
    Write-Host ""
    Write-Host "The service is already running as '$originalServiceAccount'. No change needed." -ForegroundColor Yellow
    exit 0
}

# For gMSA, verify it's installed on this machine
if ($isGMSA) {
    Write-Host ""
    Write-Host "=== Validating gMSA Account ===" -ForegroundColor Cyan
    
    if ($SkipValidation) {
        Write-Host "Skipping gMSA validation (SkipValidation flag set)" -ForegroundColor Yellow
    } else {
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
            Write-Host "Or use -SkipValidation to bypass this check if you're confident the gMSA is configured correctly." -ForegroundColor Yellow
            exit 1
        }
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

# Rollback tracking - record what was newly granted so we can revert on failure
$foldersGranted = @()
$rightsGranted = @()
$groupsGranted = @()
$serviceAccountChanged = $false

# Folder paths for permission checks and rollback (used by both grant and Invoke-Rollback).
# Use Windows environment variables so these work on non-default drive/path installations.
# $env:ProgramW6432 is the 64-bit Program Files path (set in WOW64); fall back to $env:ProgramFiles
# in 64-bit PowerShell sessions where $env:ProgramW6432 may not be set.
$programFiles64 = if ($env:ProgramW6432) { $env:ProgramW6432 } else { $env:ProgramFiles }
$agentFolders = @(
    "$env:ProgramData\New Relic\newrelic-infra",
    "$programFiles64\New Relic\newrelic-infra"
)

if (-not $isGMSA -and -not $isBuiltinTarget) {

    # Check if user exists (local or domain) - skip for gMSA and built-in accounts
    $userExists = Get-LocalUser -Name $usernameOnly -ErrorAction SilentlyContinue
    if (-not $userExists) {
        Write-Host "User '$usernameOnly' not found as local user. Verifying domain account..." -ForegroundColor Yellow
        try {
            $account = New-Object System.Security.Principal.NTAccount($Username)
            $account.Translate([System.Security.Principal.SecurityIdentifier]) | Out-Null
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
            return ($null -ne $userAccess)
        } catch {
            Write-Warning "Could not check permissions for $Path"
            return $false
        }
    }

    foreach ($folder in $agentFolders) {
        if (-not (Test-UserFolderPermission -Path $folder -User $usernameOnly)) {
            Write-Warning "User does not have Full Control on: $folder"
            Write-Host "  Granting permissions..." -ForegroundColor Yellow
            try {
                icacls "$folder" /grant "${usernameOnly}:(OI)(CI)F" /T /Q | Out-Null
                $foldersGranted += $folder
                Write-Host "  Permissions granted on $folder" -ForegroundColor Green
            } catch {
                Write-Error "Failed to grant permissions on $folder"
                exit 1
            }
        } else {
            Write-Host "User has permissions on: $folder" -ForegroundColor Green
        }
    }
}

# Grant user rights for both regular accounts and gMSA
Write-Host ""
Write-Host "Checking user privileges..." -ForegroundColor Cyan

function Resolve-AccountSid {
    param([string]$AccountName)
    try {
        return (New-Object System.Security.Principal.NTAccount($AccountName)).Translate([System.Security.Principal.SecurityIdentifier]).Value
    } catch {
        # Fallback for gMSA: NTAccount.Translate can fail when LSA cannot resolve
        # managed service accounts. Use DirectorySearcher as an alternative.
        $samName = $AccountName -replace '^.*\\', ''
        try {
            $searcher = New-Object System.DirectoryServices.DirectorySearcher
            $searcher.Filter = "(sAMAccountName=$samName)"
            $searcher.PropertiesToLoad.Add('objectSid') | Out-Null
            $result = $searcher.FindOne()
            if ($result) {
                $sidBytes = $result.Properties['objectsid'][0]
                return (New-Object System.Security.Principal.SecurityIdentifier($sidBytes, 0)).Value
            }
        } catch {}
        throw "Could not resolve SID for account: $AccountName"
    }
}

# Opens a local LSA policy handle. Access: POLICY_LOOKUP_NAMES (0x800) | POLICY_CREATE_ACCOUNT (0x10)
function Open-LsaPolicy {
    $objAttr = New-Object LSAUtils.LSA_OBJECT_ATTRIBUTES
    $objAttr.Length = [System.Runtime.InteropServices.Marshal]::SizeOf($objAttr)
    $sysName = New-Object LSAUtils.LSA_UNICODE_STRING
    [IntPtr]$handle = [IntPtr]::Zero
    $status = [LSAUtils.NativeMethods]::LsaOpenPolicy([ref]$sysName, [ref]$objAttr, 0x810, [ref]$handle)
    if ($status -ne 0) { throw "LsaOpenPolicy failed: NTSTATUS 0x$($status.ToString('X8'))" }
    return $handle
}

# Resolves an account name to an unmanaged SID pointer (caller must FreeHGlobal when done)
function Get-AccountSidPtr {
    param([string]$AccountName)
    $sidStr = Resolve-AccountSid -AccountName $AccountName
    $sid = New-Object System.Security.Principal.SecurityIdentifier($sidStr)
    $sidBytes = New-Object byte[] $sid.BinaryLength
    $sid.GetBinaryForm($sidBytes, 0)
    $ptr = [System.Runtime.InteropServices.Marshal]::AllocHGlobal($sidBytes.Length)
    [System.Runtime.InteropServices.Marshal]::Copy($sidBytes, 0, $ptr, $sidBytes.Length)
    return $ptr
}

function Grant-UserRights {
    param([string]$User, [hashtable]$Rights)
    $sidPtr = Get-AccountSidPtr -AccountName $User
    try {
        $policy = Open-LsaPolicy
        try {
            $existingRights = [LSAUtils.NativeMethods]::GetAccountRights($policy, $sidPtr)
            $newlyAdded = @()
            foreach ($right in $Rights.Keys) {
                if ($existingRights -contains $right) {
                    Write-Host "  Already has: $($Rights[$right])" -ForegroundColor Gray
                } else {
                    Write-Host "  Granting: $($Rights[$right])" -ForegroundColor Yellow
                    $status = [LSAUtils.NativeMethods]::AddRights($policy, $sidPtr, [string[]]@($right))
                    if ($status -ne 0) {
                        Write-Warning "  Failed to grant '$($Rights[$right])': NTSTATUS 0x$($status.ToString('X8'))"
                    } else {
                        $newlyAdded += $right
                        Write-Host "    ✓ Granted: $($Rights[$right])" -ForegroundColor Green
                    }
                }
            }
            return $newlyAdded
        } finally {
            [LSAUtils.NativeMethods]::LsaClose($policy) | Out-Null
        }
    } catch {
        Write-Error "Failed to grant user rights: $_"
        return @()
    } finally {
        [System.Runtime.InteropServices.Marshal]::FreeHGlobal($sidPtr)
    }
}

$requiredRights = @{
    "SeServiceLogonRight" = "Log on as a service"
    "SeDebugPrivilege" = "Debug programs"
    "SeBackupPrivilege" = "Back up files and directories"
    "SeRestorePrivilege" = "Restore files and directories"
}

# Grant-LocalGroupMembership adds the account to each group in $Groups if not already a member.
# Returns an array of group names that were newly joined (for rollback tracking).
function Grant-LocalGroupMembership {
    param([string]$User, [string[]]$Groups)
    $newlyAdded = @()
    foreach ($group in $Groups) {
        try {
            $members = Get-LocalGroupMember -Group $group -ErrorAction Stop | Select-Object -ExpandProperty Name
            if ($members -icontains $User) {
                Write-Host "  Already member of: $group" -ForegroundColor Gray
            } else {
                Add-LocalGroupMember -Group $group -Member $User -ErrorAction Stop
                $newlyAdded += $group
                Write-Host "  ✓ Added to group: $group" -ForegroundColor Green
            }
        } catch {
            Write-Warning "  Could not add '$User' to '$group': $_"
        }
    }
    return $newlyAdded
}

# Revoke-LocalGroupMembership removes the account from each group in $Groups.
function Revoke-LocalGroupMembership {
    param([string]$User, [string[]]$Groups)
    if (-not $Groups -or $Groups.Count -eq 0) { return }
    foreach ($group in $Groups) {
        try {
            Remove-LocalGroupMember -Group $group -Member $User -ErrorAction Stop
            Write-Host "  Removed from group: $group" -ForegroundColor Yellow
        } catch {
            Write-Warning "  Could not remove '$User' from '$group': $_"
        }
    }
}

$requiredGroups = @('Performance Monitor Users', 'Performance Log Users', 'Event Log Readers')

if ($isBuiltinTarget) {
    $rightsGranted = @()
    $groupsGranted = @()
} else {
    $rightsGranted = Grant-UserRights -User $usernameForRights -Rights $requiredRights

    Write-Host ""
    Write-Host "Checking local group memberships..." -ForegroundColor Cyan
    $groupsGranted = Grant-LocalGroupMembership -User $usernameOnly -Groups $requiredGroups
}

function Revoke-UserRights {
    param([string]$User, [string[]]$Rights)
    if (-not $Rights -or $Rights.Count -eq 0) { return }
    $sidPtr = Get-AccountSidPtr -AccountName $User
    try {
        $policy = Open-LsaPolicy
        try {
            $existingRights = [LSAUtils.NativeMethods]::GetAccountRights($policy, $sidPtr)
            foreach ($right in $Rights) {
                if ($existingRights -contains $right) {
                    $status = [LSAUtils.NativeMethods]::RemoveRights($policy, $sidPtr, [string[]]@($right))
                    if ($status -ne 0) {
                        Write-Warning "  Failed to revoke '$($requiredRights[$right])': NTSTATUS 0x$($status.ToString('X8'))"
                    } else {
                        Write-Host "  Reverted: $($requiredRights[$right])" -ForegroundColor Yellow
                    }
                }
            }
        } finally {
            [LSAUtils.NativeMethods]::LsaClose($policy) | Out-Null
        }
    } catch {
        Write-Warning "Could not revert user rights: $_"
    } finally {
        [System.Runtime.InteropServices.Marshal]::FreeHGlobal($sidPtr)
    }
}

function Start-AgentService {
    param([switch]$SilentOnFailure)
    Write-Host "Starting service..." -ForegroundColor Yellow
    try {
        Start-Service -Name $ServiceName -ErrorAction Stop
        Start-Sleep -Seconds 2
        $serviceStatus = Get-Service -Name $ServiceName
        if ($serviceStatus.Status -eq 'Running') {
            Write-Host "✓ Service started successfully" -ForegroundColor Green
            return $true
        } else {
            Write-Warning "Service is in state: $($serviceStatus.Status)"
            return $false
        }
    } catch {
        if ($SilentOnFailure) {
            Write-Warning "Could not restart service: $_"
        } else {
            Write-Error "Failed to start service: $_"
        }
        return $false
    }
}

function Invoke-Rollback {
    Write-Host ""
    Write-Host "=== Rolling Back Changes ===" -ForegroundColor Red

    # Revert service account back to original if it was changed
    if ($serviceAccountChanged) {
        Write-Host "  Reverting service account to: $originalServiceAccount" -ForegroundColor Yellow
        try {
            $revertWMI = Get-WmiObject -Class Win32_Service -Filter "Name='$ServiceName'"
            # Use stored credential for custom accounts; $null password for built-in accounts
            $revertPassword = if ($originalCredential) { $originalCredential.GetNetworkCredential().Password } else { $null }
            $revertResult = $revertWMI.Change($null, $null, $null, $null, $null, $null, $originalServiceAccount, $revertPassword, $null, $null, $null)
            if ($revertResult.ReturnValue -eq 0) {
                Write-Host "  ✓ Service account reverted to: $originalServiceAccount" -ForegroundColor Green
            } else {
                Write-Warning "  Could not revert service account automatically (WMI error: $($revertResult.ReturnValue))"
                Write-Host "  Manual step required: set service '$ServiceName' account back to: $originalServiceAccount" -ForegroundColor Yellow
            }
        } catch {
            Write-Warning "  Could not revert service account: $_"
            Write-Host "  Manual step required: set service '$ServiceName' account back to: $originalServiceAccount" -ForegroundColor Yellow
        }
    }

    # Revoke folder permissions that were newly granted by this script
    if ($foldersGranted.Count -gt 0) {
        foreach ($folder in $foldersGranted) {
            Write-Host "  Revoking folder permissions on: $folder" -ForegroundColor Yellow
            icacls "$folder" /remove "$usernameOnly" /T /Q | Out-Null
        }
    } else {
        Write-Host "  No folder permissions were granted by this script - skipping folder rollback" -ForegroundColor Gray
    }

    # Only revoke rights that were newly granted in this run — same as folder permissions:
    # if the account already had the right before this script ran, leave it alone.
    Revoke-UserRights -User $usernameForRights -Rights $rightsGranted

    # Remove group memberships that were newly granted in this run
    Revoke-LocalGroupMembership -User $usernameOnly -Groups $groupsGranted

    Write-Host "Attempting to restart service with original account..." -ForegroundColor Yellow
    Start-AgentService -SilentOnFailure

    Write-Host "Rollback completed." -ForegroundColor Yellow
}

Write-Host ""
Write-Host "=== Changing Service Account ===" -ForegroundColor Cyan

Write-Host "Stopping service..." -ForegroundColor Yellow
try {
    Stop-Service -Name $ServiceName -Force -ErrorAction Stop
    Write-Host "✓ Service stopped" -ForegroundColor Green
} catch {
    Write-Error "Could not stop service: $_"
    Invoke-Rollback
    exit 1
}

Write-Host "Updating service account to: $Username" -ForegroundColor Yellow
try {
    $serviceWMI = Get-WmiObject -Class Win32_Service -Filter "Name='$ServiceName'"

    # gMSA and built-in accounts (LocalSystem, NT AUTHORITY\*) require $null password - WMI returns error 22 with ""
    if ($isGMSA -or $isBuiltinTarget) {
        $result = $serviceWMI.Change($null, $null, $null, $null, $null, $null, $Username, $null, $null, $null, $null)
    } else {
        $result = $serviceWMI.Change($null, $null, $null, $null, $null, $null, $credential.UserName, $credential.GetNetworkCredential().Password, $null, $null, $null)
    }

    if ($result.ReturnValue -ne 0) {
        throw "WMI error code: $($result.ReturnValue)"
    }
    $serviceAccountChanged = $true
    Write-Host "✓ Service account updated successfully" -ForegroundColor Green
} catch {
    Write-Error "Failed to change service account: $_"
    Write-Host "Attempting to restart service with original account..." -ForegroundColor Yellow
    Start-AgentService -SilentOnFailure
    Invoke-Rollback
    exit 1
}

if (-not (Start-AgentService)) {
    Write-Host "The service account has been changed, but the service failed to start." -ForegroundColor Yellow
    Write-Host "Please check the Windows Event Logs for details." -ForegroundColor Yellow
    Invoke-Rollback
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
