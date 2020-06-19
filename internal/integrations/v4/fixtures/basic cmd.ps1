Param
(
    [parameter(Position=0, Mandatory=$false)]
    [String]
    $Argument
)

echo "stdout line"
$host.ui.WriteErrorLine('error line')
echo "$env:PREFIX-$Argument"

