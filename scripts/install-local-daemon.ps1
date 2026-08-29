# Register + start genesis node via Task Scheduler COM (current user, no 10h cap).
$ErrorActionPreference = "Stop"
$Script = Join-Path $PSScriptRoot "start-local-daemon.ps1"
$TaskName = "VEIL-local-genesis-node"

$svc = New-Object -ComObject Schedule.Service
$svc.Connect()
$folder = $svc.GetFolder("\")
$task = $svc.NewTask(0)
$task.RegistrationInfo.Description = "VEIL local stack ensure: avalanchego + anvil rails + router + frontend + mesh + tape. Idempotent; does not kill healthy processes."
$task.Settings.Enabled = $true
$task.Settings.Hidden = $true
$task.Settings.AllowDemandStart = $true
$task.Settings.DisallowStartIfOnBatteries = $false
$task.Settings.StopIfGoingOnBatteries = $false
$task.Settings.StartWhenAvailable = $true
$task.Settings.ExecutionTimeLimit = "PT0S"
$task.Settings.MultipleInstances = 0
$task.Principal.LogonType = 3
$task.Principal.RunLevel = 0

$trigger = $task.Triggers.Create(9)
$trigger.Enabled = $true
$trigger.UserId = "$env:USERDOMAIN\$env:USERNAME"

$action = $task.Actions.Create(0)
$action.Path = "powershell.exe"
$action.Arguments = "-NoProfile -ExecutionPolicy Bypass -File `"$Script`""
$action.WorkingDirectory = $PSScriptRoot

$folder.RegisterTaskDefinition($TaskName, $task, 6, $null, $null, 3) | Out-Null
$registered = $folder.GetTask($TaskName)
$registered.Run($null) | Out-Null
Write-Host "started scheduled task $TaskName (ONLOGON, no time limit)"
Write-Host "log: $(Join-Path (Split-Path $PSScriptRoot -Parent) '.local\logs\daemon-boot.log')"
