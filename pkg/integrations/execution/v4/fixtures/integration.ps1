Param
(
    [parameter(Position=0, Mandatory=$false)]
    [String]
    $Argument
)

$Argument = $Argument -Replace '\\','/' # Avoid later deserialization problems when receiving a windows path

echo (-join('{"name":"com.newrelic.test","protocol_version":"1","integration_version":"1.0.0","metrics":[{"event_type":"TestSample","value":"', $Argument, '"}]}'))
