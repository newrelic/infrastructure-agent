Param
(
    [parameter(Position=0, Mandatory=$false)]
    [String]
    $Argument
)

Write-Output "stdout line";
$host.ui.WriteErrorLine('error line')
Write-Output "$env:PREFIX-$Argument";
