### Scope
This document share which are the commands and validations needed to pass all acceptance criteria for the manual system acceptance test.
It covers support for use case of:
* Windows 64-bits, 32-bits, 
* Zip and msi installation

### Setup
- [Windows 64-bits](http://nr-downloads-ohai-staging.s3-website-us-east-1.amazonaws.com/infrastructure_agent/windows/newrelic-infra.msi)
- [Windows 32-bits](http://nr-downloads-ohai-staging.s3-website-us-east-1.amazonaws.com/infrastructure_agent/windows/386/newrelic-infra-386.msi)
- [Zip installation 32-bits](http://nr-downloads-ohai-staging.s3-website-us-east-1.amazonaws.com/infrastructure_agent/binaries/windows/386/newrelic-infra-386.1.16.0.zip)
- [Zip installation 64-bits](http://nr-downloads-ohai-staging.s3-website-us-east-1.amazonaws.com/infrastructure_agent/binaries/windows/amd64/newrelic-infra-amd64.1.16.0.zip)

### Test suit
#### Scenario 1. Package naming follow dist convention
Lookup at WIN repository folder and compare package under test with the previous package version name.
* [msi WIN Repository folder](http://nr-downloads-ohai-staging.s3-website-us-east-1.amazonaws.com/infrastructure_agent/windows/) \
package under test version: 1.16.1, name: newrelic-infra.1.16.1.msi \
previous package version: 1.16.0, name: newrelic-infra.1.16.0.msi

* [zip WIN Repository folder](http://nr-downloads-ohai-staging.s3-website-us-east-1.amazonaws.com/infrastructure_agent/binaries/windows) \
package under test version: 1.16.1, name: newrelic-infra-amd64.1.16.1.zip \
previous package version: 1.16.0, name: newrelic-infra-amd64.1.16.0.zip

#### Scenario 2. Package contains all required files
Review file managed by package and compare with previous package version.
```shell script
$ dir -r  | % { if ($_.PsIsContainer) { $_.FullName + "\" } else { $_.FullName } }
```
e.g: Win zip installation output:
```shell script
C:\Program Files\New Relic\newrelic-infra\custom-integrations\
C:\Program Files\New Relic\newrelic-infra\integrations.d\
C:\Program Files\New Relic\newrelic-infra\newrelic-integrations\
C:\Program Files\New Relic\newrelic-infra\LICENSE.txt
C:\Program Files\New Relic\newrelic-infra\newrelic-infra-ctl.exe
C:\Program Files\New Relic\newrelic-infra\newrelic-infra-service.exe
C:\Program Files\New Relic\newrelic-infra\newrelic-infra.exe
C:\Program Files\New Relic\newrelic-infra\newrelic-infra.log
C:\Program Files\New Relic\newrelic-infra\newrelic-infra.yml
C:\Program Files\New Relic\newrelic-infra\yamlgen.exe
```
e.g: Win msi installation output:
```
C:\Program Files\New Relic\newrelic-infra\custom-integrations\
C:\Program Files\New Relic\newrelic-infra\integrations.d\
C:\Program Files\New Relic\newrelic-infra\logging.d\
C:\Program Files\New Relic\newrelic-infra\newrelic-integrations\
C:\Program Files\New Relic\newrelic-infra\LICENSE.txt
C:\Program Files\New Relic\newrelic-infra\newrelic-infra-ctl.exe
C:\Program Files\New Relic\newrelic-infra\newrelic-infra-service.exe
C:\Program Files\New Relic\newrelic-infra\newrelic-infra.exe
C:\Program Files\New Relic\newrelic-infra\newrelic-infra.log
C:\Program Files\New Relic\newrelic-infra\newrelic-infra.yml
C:\Program Files\New Relic\newrelic-infra\yamlgen.exe
C:\Program Files\New Relic\newrelic-infra\integrations.d\newrelic-infra-winpkg-config.yml
C:\Program Files\New Relic\newrelic-infra\logging.d\file.yml.example
C:\Program Files\New Relic\newrelic-infra\logging.d\fluentbit.yml.example
C:\Program Files\New Relic\newrelic-infra\logging.d\logs-test.yml
C:\Program Files\New Relic\newrelic-infra\logging.d\tcp.yml.example
C:\Program Files\New Relic\newrelic-infra\logging.d\winlog.yml.example
C:\Program Files\New Relic\newrelic-infra\newrelic-integrations\logging\
C:\Program Files\New Relic\newrelic-infra\newrelic-integrations\newrelic-infra-winpkg-definition.yml
C:\Program Files\New Relic\newrelic-infra\newrelic-integrations\nr-winpkg.exe
C:\Program Files\New Relic\newrelic-infra\newrelic-integrations\nri-prometheus.exe
C:\Program Files\New Relic\newrelic-infra\newrelic-integrations\logging\fb.db
C:\Program Files\New Relic\newrelic-infra\newrelic-integrations\logging\fb.db-shm
C:\Program Files\New Relic\newrelic-infra\newrelic-integrations\logging\fb.db-wal
C:\Program Files\New Relic\newrelic-infra\newrelic-integrations\logging\fluent-bit.dll
C:\Program Files\New Relic\newrelic-infra\newrelic-integrations\logging\fluent-bit.exe
C:\Program Files\New Relic\newrelic-infra\newrelic-integrations\logging\out_newrelic.dll
C:\Program Files\New Relic\newrelic-infra\newrelic-integrations\logging\parsers.conf
```

#### Scenario 3. Check agent version
Check if version number is well inform.
```shell script
$ [System.Diagnostics.FileVersionInfo]::GetVersionInfo("C:\Program Files\New Relic\newrelic-infra\newrelic-infra.exe").FileVersion
```
expected output:
```shell script
> 1.16.1
```

#### Scenario 4. Service is working
Check if agent is running and sending metrics to NR.
```shell script
$ Get-Service newrelic-infra | Where-Object {$_.Status -EQ "Running"}
```
expected output: 
```shell script
Status   Name               DisplayName
------   ----               -----------
Running  newrelic-infra     New Relic Infrastructure Agent (x86)
```

Platform validation:
```
$ newrelic nrql query -a ${NR_ACCOUNT_ID} -q "SELECT count(*) from SystemSample where displayName = '${DISPLAY_NAME}'"
```
e.g. expected output: 
```
[
  {
    "agentName": "Infrastructure",
    "agentVersion": "1.16.1",
    ...
    "displayName": "win-test",
    ...
    "windowsPlatform": "Microsoft Windows 10 Pro",
  }
]
```

#### Scenario 5. Package metadata is valid
Review if basic metadata is in place. These steps are completely manual or through a [custom function](https://devblogs.microsoft.com/scripting/list-music-file-metadata-in-a-csv-and-open-in-excel-with-powershell/)

#### Scenario 6. Package signature is valid
Review if PDX key is same as PROD and valid.
```shell script
$ Get-AuthenticodeSignature -FilePath "C:\Program Files\New Relic\newrelic-infra\newrelic-infra.exe"

$ (Get-AuthenticodeSignature "newrelic-infra.exe").Status -eq 'Valid'
```
e.g.: expected output:
```shell script
Directory: C:\Program Files\New Relic\newrelic-infra
SignerCertificate                         Status                                 Path
-----------------                         ------                                 ----
<<GUID_KEY>>                          Valid                                  newrelic-infra.exe
```

#### Scenario 7. Agent privileged mode is working
Not supported

#### Scenario 8. Agent unprivileged mode is working
Not supported

#### Scenario 9. Package uninstall
```shell script
$ root > sc delete newrelic-infra
```
Platform Validation:
```shell script
$ newrelic nrql query -a ${NR_ACCOUNT_ID} -q "SELECT * from SystemSample where displayName = '${DISPLAY_NAME}' limit 1"
```
no data should be returned.

#### Scenario 10. Package upgrade
With an old agent version install, install the latest.
```shell script
$ msiexec.exe /qn /i PATH\TO\newrelic-infra.msi

$ net start newrelic-infra
```
Platform verification:
```
$ newrelic nrql query -a ${NR_ACCOUNT_ID} -q "SELECT * from SystemSample where displayName = '${DISPLAY_NAME}' limit 1"
```

#### Scenario 11. Built in Flex integration is working
Add Flex example yml file and review data in NR.
```shell script
$ (New-Object   System.Net.WebClient).DownloadFile("https://raw.githubusercontent.com/newrelic/nri-flex/master/examples/windows/windows-uptime.yml", "C:\Program Files\New Relic\newrelic-infra\integrations.d\flex-uptime.yml");
```

Platform verification:
```shell script
$ newrelic nrql query -a ${NR_ACCOUNT_ID} -q "SELECT uniques(integrationVersion) from flexStatusSample where displayName = '${DISPLAY_NAME}'"
```
e.g: expected output:
```json
[
  {
    "uniques.integrationVersion": [
      "1.4.0"
    ]
  }
]
```
<sub><sup>note: not supported for zip installation</sub></sup>

#### Scenario 11. Built in Log-forwarded integration is working
Enable verbose mode in agent configuration file and review data in NR.
```shell script
$ Rename-Item -Path " C:\Program Files\New Relic\newrelic-infra\logging.d\winlog.yml.example" -NewName "C:\Program Files\New Relic\newrelic-infra\logging.d\winlog.yml"
```
Platform verification:
```shell script
$ newrelic nrql query -a ${NR_ACCOUNT_ID} -q "SELECT count(*) from Log where displayName = '${DISPLAY_NAME}'"
```
e.g: expected output:
```json
[
    {
      "count": 31
    }
]
```
<sub><sup>note: not supported for zip installation</sub></sup>

#### Scenario 12. Built in Prometheus integration is working
Not applicable
