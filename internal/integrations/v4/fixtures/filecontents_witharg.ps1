Param
(
    [parameter(Position=0, Mandatory=$false)]
    [String]
    $Argument
)

Get-Content -Path $Argument
