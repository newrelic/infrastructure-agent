param (
    # Target architecture: amd64 (default) or 386
    [ValidateSet("amd64", "386")]
    [string]$arch="amd64",
    [string]$tag="v0.0.0",
    [string]$pfx_passphrase="none",
    # nri-flex
    [string]$nriFlexVersion,
    #fluent-bit
    [string]$nrfbArtifactVersion,
    #nri-winservices
    [string]$nriWinServicesVersion,
    # Signing tool
    [string]$signtool='"C:\Program Files (x86)\Windows Kits\10\bin\x64\signtool.exe"'
)

$version=$tag.substring(1)

echo "===> Import .pfx certificate from GH Secrets"
Import-PfxCertificate -FilePath ..\..\mycert.pfx -Password (ConvertTo-SecureString -String $pfx_passphrase -AsPlainText -Force) -CertStoreLocation Cert:\CurrentUser\My

echo "===> Show certificate installed"
Get-ChildItem -Path cert:\CurrentUser\My\

echo "===> Download main infra agent binaries from GH release"
$file = "newrelic-infra_binaries_windows_${version}_${arch}.zip"
write-host "URL to download:" "https://github.com/jportasa/infrastructure-agent/releases/download/v${version}/${file}"
$url = "https://github.com/jportasa/infrastructure-agent/releases/download/v${version}/${file}"

Invoke-WebRequest $url -OutFile $file
Expand-Archive $file -DestinationPath "..\..\target\bin\windows_$arch\"
ls "..\..\target\bin\windows_$arch\"

echo "===> Embedding external components"
echo "===> Embeding Flex $arch"
[string]$release="v${nriFlexVersion}"
[string]$file="nri-flex_${nriFlexVersion}_Windows_x86_64.zip"
$ProgressPreference = 'SilentlyContinue'
[Net.ServicePointManager]::SecurityProtocol = [Net.SecurityProtocolType]::Tls12
write-host "https://github.com/newrelic/nri-flex/releases/download/$release/$file"
Invoke-WebRequest "https://github.com/newrelic/nri-flex/releases/download/$release/$file" -OutFile "..\..\target\nri-flex.zip"
# embed:
$flexPath = "..\..\target\nri-flex"
$nraPath = "..\..\target\bin\windows_$arch"
# extract
New-Item -path $flexPath -type directory -Force
expand-archive -path '..\..\target\nri-flex.zip' -destinationpath $flexPath
Remove-Item '..\..\target\nri-flex.zip'
# flex binaries
Copy-Item -Path "$flexPath\nri-flex.exe" -Destination "$nraPath" -Force
# nrjmx
#Copy-Item -Path "$flexPath\nrjmx" -Destination "$nraPath\" -Recurse -Force
# clean
Remove-Item -Path $flexPath -Force -Recurse

# FluentBit
#$includeFluentBit = (
#    -Not [string]::IsNullOrWhitespace($artifactoryToken))
[bool] $includeFluentBit = 1
if ($includeFluentBit) {
    echo "===> Embeding Fluentbit $arch"
    $fbArch = "win64"
#    if($arch -eq "386") {
#        $fbArch = "win32"
#    }

    # Download fluent-bit artifacts.
    $ProgressPreference = 'SilentlyContinue'
    [Net.ServicePointManager]::SecurityProtocol = [Net.SecurityProtocolType]::Tls12
    Invoke-WebRequest "https://$env:AWS_S3_FQDN/infrastructure_agent/deps/fluent-bit/windows/nrfb-$nrfbArtifactVersion-$fbArch.zip" -OutFile nrfb.zip

    expand-archive -path '.\nrfb.zip' -destinationpath '.'
    Remove-Item -Force .\nrfb.zip
    if (-Not $skipSigning) {
        iex "& $signtool sign /d 'New Relic Infrastructure Agent' /n 'Contoso'  .\nrfb\fluent-bit.exe"
    }
    # Move the files to packaging.
    $nraPath = "..\..\target\bin\windows_$arch\"
    New-Item -path "$nraPath\logging" -type directory -Force
    Copy-Item -Path ".\nrfb\*" -Destination "$nraPath\logging" -Recurse -Force
    Remove-Item -Path ".\nrfb" -Force -Recurse
    ls "$nraPath\logging"
}

[bool] $includeWinPkg = 1
if ($includeWinPkg) {
    echo "===> Embeding Winpkg $arch"
    $WinPkgArch = "amd64"
    if($arch -eq "386") {
        $WinPkgArch = "386"
    }
    # Download WinPkg artifacts.
    $ProgressPreference = 'SilentlyContinue'
    [Net.ServicePointManager]::SecurityProtocol = [Net.SecurityProtocolType]::Tls12
    Invoke-WebRequest "https://$env:AWS_S3_FQDN/infrastructure_agent/deps/nr-winpkg/$WinPkgArch/nr-winpkg-$WinPkgArch.zip" -OutFile nr-winpkg.zip
    expand-archive -path '.\nr-winpkg.zip' -destinationpath '.\nr-winpkg'
    Remove-Item -Force .\nr-winpkg.zip
    $nraPath = "..\..\external_content\windows\$WinPkgArch"
    New-Item -path "$nraPath\winpkg" -type directory -Force
    Copy-Item -Path ".\nr-winpkg\*" -Destination "$nraPath\winpkg" -Recurse -Force
    Remove-Item -Path ".\nr-winpkg" -Force -Recurse
    ls "$nraPath\winpkg"
}

[bool] $includeYamlGen = 1
if ($includeYamlGen) {
    echo "===> Embeding yamlgen $arch"
    $ProgressPreference = 'SilentlyContinue'
    [Net.ServicePointManager]::SecurityProtocol = [Net.SecurityProtocolType]::Tls12
    Invoke-WebRequest "https://$env:AWS_S3_FQDN/infrastructure_agent/deps/yamlgen/yamlgen.exe" -OutFile yamlgen.exe
    Copy-Item -Path "yamlgen.exe" -Destination "..\..\target\bin\windows_$arch\" -Recurse -Force
    Remove-Item -Path "yamlgen.exe" -Force -Recurse
    ls "..\..\target\bin\windows_$arch\"
}

# embded nri-winservices
if (-Not [string]::IsNullOrWhitespace($nriWinServicesVersion)) {
    # download
    [string]$file="nri-winservices-${nriWinServicesVersion}-amd64.zip"
    $ProgressPreference = 'SilentlyContinue'
    [Net.ServicePointManager]::SecurityProtocol = [Net.SecurityProtocolType]::Tls12
    Invoke-WebRequest "https://github.com/newrelic/nri-winservices/releases/download/${nriWinServicesVersion}/${file}" -OutFile "nri-winservices.zip"
    # extract
    $windowsTargetPath = "..\..\target\bin\windows_$arch\nri-winservices"
    New-Item -path $windowsTargetPath -type directory -Force
    expand-archive -path 'nri-winservices.zip' -destinationpath $windowsTargetPath
    Remove-Item 'nri-winservices.zip'
    if (-Not $skipSigning) {
        iex "& $signtool sign /d 'New Relic Infrastructure Agent' /n 'Contoso'  $windowsTargetPath\nri-winservices.exe"
        iex "& $signtool sign /d 'New Relic Infrastructure Agent' /n 'Contoso'  $windowsTargetPath\windows_exporter.exe"
    }
}

$msBuild = (Get-ItemProperty hklm:\software\Microsoft\MSBuild\ToolsVersions\4.0).MSBuildToolsPath
if ($msBuild.Length -eq 0) {
    echo "Can't find MSBuild tool. .NET Framework 4.0.x must be installed"
    exit -1
}
echo $msBuild

echo "===> Create msi"
$env:path = "$env:path;C:\Program Files (x86)\Microsoft Visual Studio\2019\Enterprise\MSBuild\Current\Bin"
Push-Location -Path "..\..\build\package\windows\newrelic-infra-$arch-installer\newrelic-infra"
. $msBuild/MSBuild.exe newrelic-infra-installer.wixproj /p:AgentVersion=${version} /p:IncludeFluentBit=$includeFluentBit

echo "===>Making versioned installed copy"
cd bin\Release
cp newrelic-infra-$arch.msi newrelic-infra-${arch}.${version}.msi
Pop-Location
