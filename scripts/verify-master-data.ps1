$ErrorActionPreference = 'Stop'

$repoRoot = (Resolve-Path -LiteralPath (Join-Path $PSScriptRoot '..')).Path
foreach ($requiredPath in @('go.mod', 'Taskfile.yml', 'migrations', 'internal')) {
    if (-not (Test-Path -LiteralPath (Join-Path $repoRoot $requiredPath))) {
        throw "Verifier path does not look like the API repository: missing $requiredPath"
    }
}

$requiredTools = @('initdb', 'pg_ctl', 'createdb', 'task', 'go')
$toolPaths = @{}
foreach ($tool in $requiredTools) {
    $command = Get-Command $tool -ErrorAction SilentlyContinue
    if (-not $command) {
        throw "Master-data verifier requires '$tool' on PATH. No database was started or modified."
    }
    $toolPaths[$tool] = $command.Source
}

$tmpRoot = [IO.Path]::GetFullPath((Join-Path $repoRoot ('tmp\master-data-verifier-' + [guid]::NewGuid().ToString('N'))))
$tmpPrefix = [IO.Path]::GetFullPath((Join-Path $repoRoot 'tmp')).TrimEnd('\') + '\'
if (-not $tmpRoot.StartsWith($tmpPrefix, [StringComparison]::OrdinalIgnoreCase)) {
    throw 'Verifier temporary directory is outside the repository tmp directory.'
}
$dataPath = Join-Path $tmpRoot 'postgres-data'
$mainEnvPath = Join-Path $tmpRoot 'master-data.env'
$fkEnvPath = Join-Path $tmpRoot 'master-data-fk.env'
$postgresStarted = $false
$previousDBEnvFile = $env:DB_ENV_FILE
$databaseEnvironmentKeys = @('DB_HOST', 'DB_PORT', 'DB_NAME', 'DB_USER', 'DB_PASSWORD', 'DB_SSLMODE', 'DB_TIMEZONE')
$previousDatabaseEnvironment = @{}
foreach ($key in $databaseEnvironmentKeys) {
    $previousDatabaseEnvironment[$key] = [Environment]::GetEnvironmentVariable($key, 'Process')
    Remove-Item -LiteralPath ("Env:" + $key) -ErrorAction SilentlyContinue
}
$previousTestDatabaseURL = $env:KOLABORIA_TEST_DATABASE_URL
$previousFKDatabaseURL = $env:KOLABORIA_TEST_FK_DATABASE_URL
$previousScenario = $env:KOLABORIA_TEST_SCENARIO
$previousFKScenario = $env:KOLABORIA_TEST_FK_SCENARIO
$previousSchemaScenario = $env:KOLABORIA_TEST_SCHEMA_SCENARIO
$previousGoCache = $env:GOCACHE
$locationPushed = $false

function Invoke-CheckedCommand([string]$Executable, [string[]]$Arguments) {
    & $Executable @Arguments
    if ($LASTEXITCODE -ne 0) {
        throw "Command failed (exit $LASTEXITCODE): $Executable $($Arguments -join ' ')"
    }
}

function Get-MigrationVersion([string]$Executable, [string[]]$Arguments) {
    $output = & $Executable @Arguments 2>&1
    if ($LASTEXITCODE -ne 0) {
        throw "Command failed (exit $LASTEXITCODE): $Executable $($Arguments -join ' ')`n$($output -join "`n")"
    }
    $versionLine = @($output | Where-Object { "$_" -match '^\s*2\s*$' -or "$_" -match '^\s*2\s+false\s*$' })
    if ($versionLine.Count -ne 1 -or ($output -join "`n") -match '(?i)dirty') {
        throw "Expected clean migration version 2; got:`n$($output -join "`n")"
    }
    Write-Output ($output -join "`n")
}

try {
    Push-Location -LiteralPath $repoRoot
    $locationPushed = $true
    New-Item -ItemType Directory -Path $tmpRoot | Out-Null
    $listener = [Net.Sockets.TcpListener]::new([Net.IPAddress]::Loopback, 0)
    $listener.Start()
    $port = ([Net.IPEndPoint]$listener.LocalEndpoint).Port
    $listener.Stop()

    Invoke-CheckedCommand $toolPaths['initdb'] @('-D', $dataPath, '-U', 'master_data_test', '-A', 'trust', '--encoding=UTF8', '--no-locale')
    Invoke-CheckedCommand $toolPaths['pg_ctl'] @('-D', $dataPath, '-o', "-h 127.0.0.1 -p $port -c listen_addresses=127.0.0.1", '-l', (Join-Path $tmpRoot 'postgres.log'), 'start')
    $postgresStarted = $true

    $mainDatabaseName = 'master_data_test'
    $fkDatabaseName = 'master_data_fk_test'
    Invoke-CheckedCommand $toolPaths['createdb'] @('-h', '127.0.0.1', '-p', "$port", '-U', 'master_data_test', $mainDatabaseName)
    @("DB_HOST=127.0.0.1", "DB_PORT=$port", "DB_NAME=$mainDatabaseName", 'DB_USER=master_data_test', 'DB_PASSWORD=', 'DB_SSLMODE=disable', 'DB_TIMEZONE=UTC') | Set-Content -LiteralPath $mainEnvPath -Encoding ascii
    @("DB_HOST=127.0.0.1", "DB_PORT=$port", "DB_NAME=$fkDatabaseName", 'DB_USER=master_data_test', 'DB_PASSWORD=', 'DB_SSLMODE=disable', 'DB_TIMEZONE=UTC') | Set-Content -LiteralPath $fkEnvPath -Encoding ascii

    $env:GOCACHE = Join-Path $repoRoot 'tmp\go-build'
    $env:DB_ENV_FILE = $mainEnvPath
    $env:KOLABORIA_TEST_DATABASE_URL = "postgres://master_data_test@127.0.0.1:$port/$mainDatabaseName?sslmode=disable"
    Invoke-CheckedCommand $toolPaths['task'] @('migrate:up', '--', '--timeout=60s')
    Invoke-CheckedCommand $toolPaths['task'] @('migrate:up', '--', '--timeout=60s')
    Invoke-CheckedCommand $toolPaths['task'] @('seed', '--', '--timeout=60s')
    Get-MigrationVersion $toolPaths['task'] @('migrate:version', '--', '--timeout=60s') | Out-Host
    Invoke-CheckedCommand $toolPaths['go'] @('test', './test', '-run', 'TestMasterDataRoutesAgainstPostgres|TestMasterDataSchemaHasOnlyFiveValidatedColumns|TestMasterRepositoryCountAndFetchUseOneSnapshot', '-count=1')

    $env:KOLABORIA_TEST_SCHEMA_SCENARIO = 'prepare'
    Invoke-CheckedCommand $toolPaths['go'] @('test', './test', '-run', '^TestSchemaDownPreservesUnrelatedTableAndReplays$', '-count=1')
    Invoke-CheckedCommand $toolPaths['task'] @('migrate:down', '--', '--timeout=60s')
    Invoke-CheckedCommand $toolPaths['task'] @('migrate:down', '--', '--timeout=60s')
    $env:KOLABORIA_TEST_SCHEMA_SCENARIO = 'verify'
    Invoke-CheckedCommand $toolPaths['go'] @('test', './test', '-run', '^TestSchemaDownPreservesUnrelatedTableAndReplays$', '-count=1')
    Invoke-CheckedCommand $toolPaths['task'] @('migrate:up', '--', '--timeout=60s')
    $env:KOLABORIA_TEST_SCHEMA_SCENARIO = 'cleanup'
    Invoke-CheckedCommand $toolPaths['go'] @('test', './test', '-run', '^TestSchemaDownPreservesUnrelatedTableAndReplays$', '-count=1')
    Get-MigrationVersion $toolPaths['task'] @('migrate:version', '--', '--timeout=60s') | Out-Host

    $env:KOLABORIA_TEST_SCENARIO = 'prepare-extra'
    Invoke-CheckedCommand $toolPaths['go'] @('test', './test', '-run', '^TestSeedRollbackLifecycle$', '-count=1')
    Invoke-CheckedCommand $toolPaths['task'] @('migrate:down', '--', '--timeout=60s')
    $env:KOLABORIA_TEST_SCENARIO = 'verify-extra'
    Invoke-CheckedCommand $toolPaths['go'] @('test', './test', '-run', '^TestSeedRollbackLifecycle$', '-count=1')
    Invoke-CheckedCommand $toolPaths['task'] @('migrate:up', '--', '--timeout=60s')
    $env:KOLABORIA_TEST_SCENARIO = 'cleanup-extra'
    Invoke-CheckedCommand $toolPaths['go'] @('test', './test', '-run', '^TestSeedRollbackLifecycle$', '-count=1')
    Remove-Item Env:KOLABORIA_TEST_SCENARIO -ErrorAction SilentlyContinue
    Invoke-CheckedCommand $toolPaths['go'] @('test', './test', '-run', 'TestMasterDataRoutesAgainstPostgres', '-count=1')

    Invoke-CheckedCommand $toolPaths['createdb'] @('-h', '127.0.0.1', '-p', "$port", '-U', 'master_data_test', $fkDatabaseName)
    $env:DB_ENV_FILE = $fkEnvPath
    $env:KOLABORIA_TEST_FK_DATABASE_URL = "postgres://master_data_test@127.0.0.1:$port/$fkDatabaseName?sslmode=disable"
    Invoke-CheckedCommand $toolPaths['task'] @('migrate:up', '--', '--timeout=60s')
    Invoke-CheckedCommand $toolPaths['task'] @('migrate:down', '--', '--timeout=60s')
    $env:KOLABORIA_TEST_FK_SCENARIO = 'prepare'
    Invoke-CheckedCommand $toolPaths['go'] @('test', './test', '-run', '^TestSchemaDownForeignKeyGuard$', '-count=1')

    & $toolPaths['task'] migrate:down -- --timeout=60s
    if ($LASTEXITCODE -eq 0) {
        throw 'Schema rollback unexpectedly succeeded despite a referencing foreign key.'
    }
    $env:KOLABORIA_TEST_FK_SCENARIO = 'verify'
    Invoke-CheckedCommand $toolPaths['go'] @('test', './test', '-run', '^TestSchemaDownForeignKeyGuard$', '-count=1')
    Remove-Item Env:KOLABORIA_TEST_SCHEMA_SCENARIO -ErrorAction SilentlyContinue
    Write-Output 'Master-data verifier passed on an isolated loopback PostgreSQL cluster.'
}
finally {
    if ($postgresStarted) {
        & $toolPaths['pg_ctl'] -D $dataPath -m immediate stop | Out-Null
    }
    $env:DB_ENV_FILE = $previousDBEnvFile
    foreach ($key in $databaseEnvironmentKeys) {
        $previousValue = $previousDatabaseEnvironment[$key]
        if ($null -eq $previousValue) {
            Remove-Item -LiteralPath ("Env:" + $key) -ErrorAction SilentlyContinue
        } else {
            [Environment]::SetEnvironmentVariable($key, $previousValue, 'Process')
        }
    }
    $env:KOLABORIA_TEST_DATABASE_URL = $previousTestDatabaseURL
    $env:KOLABORIA_TEST_FK_DATABASE_URL = $previousFKDatabaseURL
    $env:KOLABORIA_TEST_SCENARIO = $previousScenario
    $env:KOLABORIA_TEST_FK_SCENARIO = $previousFKScenario
    if ($null -eq $previousSchemaScenario) { Remove-Item Env:KOLABORIA_TEST_SCHEMA_SCENARIO -ErrorAction SilentlyContinue } else { $env:KOLABORIA_TEST_SCHEMA_SCENARIO = $previousSchemaScenario }
    $env:GOCACHE = $previousGoCache
    if (Test-Path -LiteralPath $tmpRoot) {
        $tmpItem = Get-Item -Force -LiteralPath $tmpRoot
        if ($tmpItem.PSIsContainer -and -not ($tmpItem.Attributes -band [IO.FileAttributes]::ReparsePoint) -and $tmpRoot.StartsWith($tmpPrefix, [StringComparison]::OrdinalIgnoreCase)) {
            Remove-Item -LiteralPath $tmpRoot -Recurse -Force
        }
    }
    if ($locationPushed) {
        Pop-Location
    }
}
