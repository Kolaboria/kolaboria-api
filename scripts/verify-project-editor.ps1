param(
    [Parameter(Mandatory = $true)]
    [string]$TestEnvFile
)

$ErrorActionPreference = 'Stop'
$repoRoot = (Resolve-Path -LiteralPath (Join-Path $PSScriptRoot '..')).Path
$envPath = (Resolve-Path -LiteralPath $TestEnvFile).Path
if (-not (Test-Path -LiteralPath $envPath -PathType Leaf)) {
    throw 'TestEnvFile must point to an existing disposable test environment file.'
}

$settings = @{}
foreach ($line in Get-Content -LiteralPath $envPath) {
    $line = $line.TrimStart([char]0xFEFF)
    if ($line -match '^\s*([A-Z_]+)\s*=\s*(.*)\s*$') {
        $settings[$matches[1]] = $matches[2].Trim().Trim('"').Trim("'")
    }
}

$databaseName = $settings['DB_NAME']
$databaseHost = $settings['DB_HOST']
$databaseUser = $settings['DB_USER']
if ($databaseHost -notin @('127.0.0.1', 'localhost') -or
    $databaseName -notmatch '^project_editor_test_[a-z0-9_]+$' -or
    $databaseUser -ne 'project_editor_test') {
    throw 'Refusing database work: use a loopback PostgreSQL target named project_editor_test_* with DB_USER=project_editor_test.'
}
foreach ($required in @('DB_PORT', 'DB_PASSWORD', 'DB_SSLMODE', 'DB_TIMEZONE')) {
    if (-not $settings.ContainsKey($required)) {
        throw "Test environment file is missing $required. No database action was run."
    }
}

$task = Get-Command task -ErrorAction SilentlyContinue
$go = Get-Command go -ErrorAction SilentlyContinue
if (-not $task -or -not $go) {
    throw 'The project-editor migration verifier requires task and go on PATH.'
}

function Invoke-Checked([string]$Executable, [string[]]$Arguments) {
    & $Executable @Arguments
    if ($LASTEXITCODE -ne 0) {
        throw "Command failed with exit code $($LASTEXITCODE): $Executable $($Arguments -join ' ')"
    }
}

function Get-CurrentMigrationVersion {
    $output = & $go.Source run ./cmd/db version --env-file $envPath 2>&1
    if ($LASTEXITCODE -ne 0) { throw 'Could not read the disposable database migration version.' }
    $matches = [regex]::Matches(($output -join "`n"), '(?m)^\s*(\d+)(?:\s+(?:false|true))?\s*$')
    if ($matches.Count -ne 1) { throw 'Could not safely parse the disposable database migration version.' }
    return [int]$matches[0].Groups[1].Value
}

$escapedPassword = [Uri]::EscapeDataString($settings['DB_PASSWORD'])
$passwordPart = if ($settings['DB_PASSWORD']) { ":$escapedPassword" } else { '' }
$testDatabaseURL = "postgres://$($settings['DB_USER'])$passwordPart@$databaseHost`:$($settings['DB_PORT'])/${databaseName}?sslmode=$($settings['DB_SSLMODE'])"
$previousEnvFile = $env:DB_ENV_FILE
$previousTestURL = $env:KOLABORIA_TEST_DATABASE_URL
try {
    Push-Location -LiteralPath $repoRoot
    $env:DB_ENV_FILE = $envPath
    $env:KOLABORIA_TEST_DATABASE_URL = $testDatabaseURL

    Invoke-Checked $task.Source @('migrate:up', '--', '--timeout=60s')
    if ((Get-CurrentMigrationVersion) -ne 11) {
        throw 'Expected migration 000011 after applying pending migrations to the disposable test database.'
    }
    Invoke-Checked $go.Source @('test', './test', '-run', '^TestProjectEditorAggregateTransactionsAgainstPostgres$', '-count=1')

    Invoke-Checked $task.Source @('migrate:down', '--', '--timeout=60s')
    if ((Get-CurrentMigrationVersion) -ne 10) {
        throw 'Expected the applicants migration to roll back before project-editor migrations.'
    }
    Invoke-Checked $task.Source @('migrate:down', '--', '--timeout=60s')
    if ((Get-CurrentMigrationVersion) -ne 9) {
        throw 'Expected rollback of migration 000010; schema version 000009 must remain.'
    }
    Invoke-Checked $task.Source @('migrate:down', '--', '--timeout=60s')
    if ((Get-CurrentMigrationVersion) -ne 8) {
        throw 'Expected rollback of migration 000009; the earlier schema version must remain 000008.'
    }
    Invoke-Checked $task.Source @('migrate:up', '--', '--timeout=60s')
    if ((Get-CurrentMigrationVersion) -ne 11) {
        throw 'Expected migrations 000009–000011 to replay cleanly on the disposable test database.'
    }
    Invoke-Checked $go.Source @('test', './test', '-run', '^TestProjectEditorAggregateTransactionsAgainstPostgres$', '-count=1')
    Write-Output 'Project editor schema replay and aggregate integration checks passed on the disposable test database.'
} finally {
    Pop-Location
    if ($null -eq $previousEnvFile) { Remove-Item Env:DB_ENV_FILE -ErrorAction SilentlyContinue } else { $env:DB_ENV_FILE = $previousEnvFile }
    if ($null -eq $previousTestURL) { Remove-Item Env:KOLABORIA_TEST_DATABASE_URL -ErrorAction SilentlyContinue } else { $env:KOLABORIA_TEST_DATABASE_URL = $previousTestURL }
}
