Param
(
    [parameter(Position=0, Mandatory=$false)]
    [String]
    $Argument
)

$Argument = $Argument -Replace '\\','/' # Avoid later deserialization problems when receiving a windows path

$err = 'time="2020-02-11T17:28:50+01:00" level=error msg="config: failed to sub lookup file data" component=integrations.runner.Group error="config name: /var/db/something: p file, error: open /var/db/something: no such file or directory" integration_name=nri-flex'
[Console]::Error.WriteLine($err)

$err = 'time="2020-02-11T17:28:52+01:00" level=fatal msg="config: fatal error" component=integrations.runner.Group error="cannot read configuration file" integration_name=nri-flex'
[Console]::Error.WriteLine($err)

echo (-join('{"name":"com.newrelic.test","protocol_version":"1","integration_version":"1.0.0","metrics":[{"event_type":"TestSample","value":"', $Argument, '"}]}'))
