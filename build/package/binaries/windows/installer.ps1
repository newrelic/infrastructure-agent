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
    [string]$ServiceOverwrite = $env:NRIA_OVERWRITE
)

# Convert ServiceOverwrite string to boolean
$ServiceOverwriteBool = $false
if ($ServiceOverwrite -eq "true" -or $ServiceOverwrite -eq "1" -or $ServiceOverwrite -eq $true) {
    $ServiceOverwriteBool = $true
}

# Set strict error handling - any unhandled error will cause script to exit
$ErrorActionPreference = "Stop"

# Initialize debug log file
# For user accounts: use <drive>:\Users\<username>\.newrelic
# For SYSTEM account (MSI): use <SystemDrive>:\ProgramData\New Relic\.logs
if ($env:USERNAME -eq "SYSTEM") {
    $tempPath = "$env:SystemDrive\ProgramData\New Relic\newrelic-infra\tmp"
} elseif ($env:USERPROFILE) {
    $tempPath = "$env:USERPROFILE\.newrelic"
} else {
    $tempPath = "$env:SystemDrive\Windows\Temp"
}
$DebugLogFile = Join-Path $tempPath "newrelic_installer_debug.log"

try {
    # Logging function
    function Write-DebugLog {
        param([string]$Message)
        $timestamp = Get-Date -Format "yyyy-MM-dd HH:mm:ss"
        $logEntry = "[$timestamp] $Message"
        try {
            # Ensure directory exists
            $logDir = Split-Path $DebugLogFile -Parent
            if (-not (Test-Path $logDir)) {
                New-Item -Path $logDir -ItemType Directory -Force | Out-Null
            }
            # Write to log file
            Add-Content -Path $DebugLogFile -Value $logEntry -Force
        } catch {
            Write-Warning "Failed to write to debug log: $_"
        }
        Write-Host $Message
    }

    # Initialize log file with startup message
    try {
        $logDir = Split-Path $DebugLogFile -Parent
        if (-not (Test-Path $logDir)) {
            New-Item -Path $logDir -ItemType Directory -Force | Out-Null
        }
        $timestamp = Get-Date -Format "yyyy-MM-dd HH:mm:ss"
        "[$timestamp] =====================================" | Out-File -FilePath $DebugLogFile -Force
        "[$timestamp] Installer script started" | Add-Content -Path $DebugLogFile -Force
        "[$timestamp] Running as user: $env:USERNAME" | Add-Content -Path $DebugLogFile -Force
        "[$timestamp] Log file: $DebugLogFile" | Add-Content -Path $DebugLogFile -Force
        "[$timestamp] =====================================" | Add-Content -Path $DebugLogFile -Force

        # Display log location prominently
        Write-Host ""
        Write-Host "========================================" -ForegroundColor Cyan
        Write-Host "Installation Debug Log: $DebugLogFile" -ForegroundColor Cyan
        Write-Host "========================================" -ForegroundColor Cyan
        Write-Host ""
    } catch {
        Write-Warning "Failed to initialize debug log file: $_"
    }

    # Check for admin rights
    function Check-Administrator {
        $user = [Security.Principal.WindowsIdentity]::GetCurrent()
        (New-Object Security.Principal.WindowsPrincipal $user).IsInRole([Security.Principal.WindowsBuiltinRole]::Administrator)
    }

    Write-DebugLog "Starting New Relic Infrastructure Agent installer"
    Write-DebugLog "Debug log file: $DebugLogFile"

    if (-Not (Check-Administrator)) {
        Write-DebugLog "ERROR: Admin permission check failed"
        Write-Error "Admin permission is required. Please, open a Windows PowerShell session with administrative rights."
        exit 1
    }
    Write-DebugLog "Admin permission check passed"

    # The priority is:
    # 1 Command-line parameter
    # 2 Environment variable
    # 3 Default value
    # Use ProgramW6432 to ensure 64-bit path even when called from 32-bit process
    $programFilesPath = if ($env:ProgramW6432) { $env:ProgramW6432 } else { $env:ProgramFiles }
    if (-Not $AgentDir)    { $AgentDir    = [IO.Path]::Combine($programFilesPath, 'New Relic\newrelic-infra') }
    if (-Not $LogFile)     { $LogFile     = [IO.Path]::Combine($AgentDir,'newrelic-infra.log') }
    if (-Not $PluginDir)   { $PluginDir   = [IO.Path]::Combine($AgentDir,'integrations.d') }
    if (-Not $ConfigFile)  { $ConfigFile  = [IO.Path]::Combine($AgentDir,'newrelic-infra.yml') }
    if (-Not $AppDataDir)  { $AppDataDir  = [IO.Path]::Combine($env:ProgramData, 'New Relic\newrelic-infra') }
    if (-Not $ServiceName) { $ServiceName = 'newrelic-infra' }

    Write-DebugLog "Configuration parameters:"
    Write-DebugLog "  AgentDir: $AgentDir"
    Write-DebugLog "  LogFile: $LogFile"
    Write-DebugLog "  PluginDir: $PluginDir"
    Write-DebugLog "  ConfigFile: $ConfigFile"
    Write-DebugLog "  AppDataDir: $AppDataDir"
    Write-DebugLog "  ServiceName: $ServiceName"
    Write-DebugLog "  ServiceOverwrite: $ServiceOverwriteBool"

    # Check if service already exists
    $existingService = Get-Service $ServiceName -ErrorAction SilentlyContinue
    $isUpgrade = $false
    $preservedAccount = $null

    Write-DebugLog "Checking for existing service: $ServiceName"
    if ($existingService) {
        Write-DebugLog "Service $ServiceName found - already exists"

        # Get the existing service account before stopping
        $existingServiceWMI = Get-WmiObject -Class Win32_Service -Filter "name='$ServiceName'"
        if ($existingServiceWMI) {
            $preservedAccount = $existingServiceWMI.StartName
            Write-DebugLog "Existing service runs as: $preservedAccount"
        }

        # Stop the service if running
        if ($existingService.Status -eq 'Running') {
            Write-DebugLog "Stopping service $ServiceName..."
            try {
                Stop-Service $ServiceName -Force -ErrorAction Stop | Out-Null
                Start-Sleep -Seconds 2
                Write-DebugLog "Service stopped successfully"
            } catch {
                Write-DebugLog "ERROR: Failed to stop service: $_"
                exit 1062
            }
        }

        if ($ServiceOverwriteBool -eq $true) {
            Write-DebugLog "ServiceOverwrite flag is set - removing existing service $ServiceName..."
            if ($existingServiceWMI) {
                $existingServiceWMI.delete() | Out-Null
                Write-DebugLog "Service removed. Performing fresh installation..."
                # Treat as fresh install, not upgrade
                $isUpgrade = $false
                $preservedAccount = $null
            } else {
                Write-DebugLog "ERROR: Failed to find service to remove"
            }
        } else {
            Write-DebugLog "ServiceOverwrite flag not set - upgrading existing service $ServiceName..."
            $isUpgrade = $true
        }
    } else {
        Write-DebugLog "Service $ServiceName not found - performing fresh installation"
    }

    function Get-ScriptDirectory {
        Split-Path -parent $PSCommandPath
    }

    $ScriptPath = Get-ScriptDirectory

    if (Test-Path "$ScriptPath\newrelic-infra.exe") {
        $versionOutput = & "$ScriptPath\newrelic-infra.exe" -version
        Write-DebugLog "Installing $versionOutput"
    }
    Write-Host -NoNewline "Using the following configuration..."
    [PSCustomObject] @{
        AgentDir         = $AgentDir
        LogFile          = $LogFile
        PluginDir        = $PluginDir
        ConfigFile       = $ConfigFile
        AppDataDir       = $AppDataDir
        ServiceName      = $ServiceName
        ServiceOverwrite = $ServiceOverwriteBool
    } | Format-List

    Function Create-Directory ($dir) {
        if (-Not (Test-Path -Path $dir)) {
            Write-DebugLog "Creating directory: $dir"
            New-Item -ItemType directory -Path $dir | Out-Null
        } else {
            Write-DebugLog "Directory already exists: $dir"
        }
    }

    # Create directories only for fresh installation
    if (-not $isUpgrade) {
        Write-DebugLog "Creating directories for fresh installation"
        Create-Directory $AgentDir
        Create-Directory $AgentDir\custom-integrations
        Create-Directory $AgentDir\newrelic-integrations
        Create-Directory $AgentDir\integrations.d
        
        # Copy LICENSE.txt only for ZIP installations
        if ($ScriptPath -ne $AgentDir) {
            Copy-Item -Path "$ScriptPath\LICENSE.txt" -Destination "$AgentDir" -Force
            Write-DebugLog "LICENSE.txt copied"
        }

        $LogDir = Split-Path -parent $LogFile
        Create-Directory $LogDir
        Create-Directory $PluginDir
        Create-Directory $AppDataDir
    } else {
        Write-DebugLog "Upgrade detected - skipping directory creation"
    }

    # Copy executables only if source and destination are different (ZIP installation)
    # For MSI installations, files are already in place
    if ($ScriptPath -ne $AgentDir) {
        Write-DebugLog "Removing old binaries from $AgentDir..."
        try {
            Remove-Item -Path "$AgentDir\*.exe" -Force -ErrorAction SilentlyContinue
            Write-DebugLog "Old binaries removed"
        } catch {
            Write-DebugLog "WARNING: Failed to remove old binaries: $_"
        }
        
        Write-DebugLog "Copying executables from $ScriptPath to $AgentDir"
        try {
            Copy-Item -Path "$ScriptPath\*.exe" -Destination "$AgentDir" -Force -ErrorAction Stop
            Write-DebugLog "Executables copied successfully"
        } catch {
            Write-DebugLog "ERROR: Failed to copy executables: $_"
            exit 5
        }
    } else {
        Write-DebugLog "Installation - files already in place, skipping copy"
        # Verify that the main executable exists
        if (-not (Test-Path "$AgentDir\newrelic-infra.exe")) {
            Write-DebugLog "ERROR: newrelic-infra.exe not found at $AgentDir after MSI file installation"
            Write-DebugLog "This indicates MSI has not yet placed files or there was an error during file installation"
            exit 2
        }
        Write-DebugLog "Verified: newrelic-infra.exe exists at $AgentDir"
    }

    # For upgrades, only update config if it doesn't exist
    if ($isUpgrade -and (Test-Path $ConfigFile)) {
        Write-DebugLog "Preserving existing configuration file: $ConfigFile"
    } else {
        Write-DebugLog "Creating new config file in $ConfigFile"
        Clear-Content -Path $ConfigFile -ErrorAction SilentlyContinue
        Add-Content -Path $ConfigFile -Value `
            "license_key: $LicenseKey",
            "log_file: $LogFile",
            "plugin_dir: $PluginDir",
            "app_data_dir: $AppDataDir"
    }

    if ($isUpgrade) {
        # Upgrade scenario: restart service with preserved account
        Write-DebugLog "Upgrade path: Restarting service with preserved account: $preservedAccount"
        
        # Grant permissions if using a custom service account (not LocalSystem)
        if ($preservedAccount -and $preservedAccount -ne "LocalSystem") {
            Write-DebugLog "Granting permissions to $preservedAccount on $AppDataDir..."
            $username = $preservedAccount -replace '^\.\\'  # Remove .\ prefix if present
            icacls "$AppDataDir" /grant "${username}:(OI)(CI)F" /T /Q | Out-Null
            Write-DebugLog "Permissions granted successfully"
        }
        
        try {
            # Ensure service is set to Automatic startup (it might be disabled)
            Write-DebugLog "Setting service startup type to Automatic..."
            Set-Service -Name $ServiceName -StartupType Automatic -ErrorAction Stop
            Write-DebugLog "Service startup type set to Automatic"
            
            Write-DebugLog "Starting service: $ServiceName"
            Start-Service -Name $ServiceName -ErrorAction Stop
            Write-DebugLog "Upgrade completed successfully!"
        } catch {
            Write-DebugLog "ERROR: Failed to restart service: $_"
            exit 1058
        }
    } else {
        # Fresh installation scenario: create service with LocalSystem
        Write-DebugLog "Creating new service: $ServiceName"
        Write-DebugLog "Binary path: $AgentDir\newrelic-infra-service.exe -config $ConfigFile"
        try {
            New-Service -Name $ServiceName -DisplayName 'New Relic Infrastructure Agent' -BinaryPathName "$AgentDir\newrelic-infra-service.exe -config $ConfigFile" -StartupType Automatic -ErrorAction Stop | Out-Null
            Write-DebugLog "Service created successfully"
        } catch {
            Write-DebugLog "ERROR: Failed to create service: $_"
            exit 1073
        }

        try {
            Write-DebugLog "Starting service: $ServiceName"
            Start-Service -Name $ServiceName -ErrorAction Stop
            Write-DebugLog "Installation completed successfully!"
        } catch {
            Write-DebugLog "ERROR: Failed to start service: $_"
            exit 1058
        }
    }

    Write-DebugLog "Installer script finished"
} catch {
    # Catch all unhandled errors and ensure script exits with non-zero code
    Write-DebugLog "FATAL ERROR: Installation failed with error: $_"
    Write-Error "Installation failed: $_"
    exit 1
}
