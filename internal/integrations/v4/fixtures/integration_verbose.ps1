
$verbosity = Get-ChildItem Env:VERBOSE
$host.ui.WriteErrorLine("VERBOSE=$($verbosity.Value)")
Write-Output "stdout line"
