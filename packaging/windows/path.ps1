param(
    [Parameter(Mandatory = $true, Position = 0)]
    [ValidateSet("add", "remove")]
    [string] $Action,

    [Parameter(Mandatory = $true, Position = 1)]
    [string] $Directory
)

$current = [Environment]::GetEnvironmentVariable("Path", "User")
$entries = @($current -split ";" | Where-Object { $_ -and $_ -ine $Directory })

if ($Action -eq "add") {
    $entries += $Directory
}

[Environment]::SetEnvironmentVariable("Path", ($entries -join ";"), "User")
