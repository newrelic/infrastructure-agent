Param
(
    [parameter(Position=0, Mandatory=$true)]
    [String]
    $Argument
)

Write-Output "sleeping $Argument seconds"
Start-Sleep -s $Argument
