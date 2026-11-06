[CmdletBinding()]
param()

Set-StrictMode -Version 2.0
$ErrorActionPreference = 'Stop'

$toolPath = Join-Path (Split-Path -Parent $PSScriptRoot) 'Generate-WYDAssetManifest.ps1'
$fixtureRoot = Join-Path $PSScriptRoot ('.asset-manifest-fixture-' + [Guid]::NewGuid().ToString('N'))

function Assert-True {
    param(
        [bool] $Condition,
        [string] $Message
    )
    if (-not $Condition) {
        throw $Message
    }
}

try {
    [void](New-Item -ItemType Directory -Path $fixtureRoot)
    [void](New-Item -ItemType Directory -Path (Join-Path $fixtureRoot 'assets'))
    [void](New-Item -ItemType Directory -Path (Join-Path $fixtureRoot '.git'))
    [void](New-Item -ItemType Directory -Path (Join-Path $fixtureRoot 'node_modules'))
    [void](New-Item -ItemType Directory -Path (Join-Path $fixtureRoot 'build'))
    [void](New-Item -ItemType Directory -Path (Join-Path $fixtureRoot 'reports'))

    Set-Content -LiteralPath (Join-Path $fixtureRoot 'assets\sample.png') -Value 'synthetic image payload' -Encoding UTF8
    Set-Content -LiteralPath (Join-Path $fixtureRoot '.env.local') -Value 'SYNTHETIC_SECRET=fixture-only' -Encoding UTF8
    Set-Content -LiteralPath (Join-Path $fixtureRoot 'service-token.json') -Value '{"synthetic":"secret"}' -Encoding UTF8
    Set-Content -LiteralPath (Join-Path $fixtureRoot 'archive.custombak') -Value 'backup fixture' -Encoding UTF8
    Set-Content -LiteralPath (Join-Path $fixtureRoot 'run.trace') -Value 'log fixture' -Encoding UTF8
    Set-Content -LiteralPath (Join-Path $fixtureRoot '.git\ignored.json') -Value '{}' -Encoding UTF8
    Set-Content -LiteralPath (Join-Path $fixtureRoot 'node_modules\ignored.js') -Value 'ignored' -Encoding UTF8
    Set-Content -LiteralPath (Join-Path $fixtureRoot 'build\ignored.bin') -Value 'ignored' -Encoding UTF8

    $jsonPath = Join-Path $fixtureRoot 'reports\manifest.json'
    $csvPath = [System.IO.Path]::ChangeExtension($jsonPath, '.csv')
    Assert-True (-not (Test-Path -LiteralPath $jsonPath)) 'Expected fresh JSON output path before first run.'
    Assert-True (-not (Test-Path -LiteralPath $csvPath)) 'Expected fresh CSV output path before first run.'
    & $toolPath `
        -ProjectRoot $fixtureRoot `
        -OutputPath $jsonPath `
        -ExcludeBackupPatterns @('*.bak', '*.custombak') `
        -ExcludeLogPatterns @('*.log', '*.trace')
    Assert-True (Test-Path -LiteralPath $jsonPath) 'Expected JSON manifest to be created.'
    Assert-True (Test-Path -LiteralPath $csvPath) 'Expected CSV manifest to be created.'

    $manifest = Get-Content -LiteralPath $jsonPath -Raw | ConvertFrom-Json
    Assert-True ($manifest.assets.Count -eq 1) 'Expected exactly one included synthetic asset.'
    $asset = $manifest.assets[0]
    Assert-True ($asset.relativePath -eq 'assets/sample.png') 'Expected a project-relative asset path.'
    Assert-True ($asset.extension -eq '.png') 'Expected the file extension to be recorded.'
    Assert-True ($asset.sizeBytes -gt 0) 'Expected a nonzero byte size.'
    $expectedHash = (Get-FileHash -LiteralPath (Join-Path $fixtureRoot 'assets\sample.png') -Algorithm SHA256).Hash.ToLowerInvariant()
    Assert-True ($asset.sha256 -eq $expectedHash) 'Expected the correct lowercase SHA-256 digest.'
    Assert-True ($asset.probableCategory -eq 'image') 'Expected image category detection.'
    Assert-True ($asset.modifiedTime -match '^\d{4}-\d{2}-\d{2}T') 'Expected a UTC modified timestamp.'
    Assert-True ($null -ne $manifest.PSObject.Properties['warnings']) 'Expected a warnings field in JSON.'
    $outputEntries = @($manifest.assets | Where-Object { $_.relativePath -like 'reports/manifest.*' })
    Assert-True ($outputEntries.Count -eq 0) 'Expected JSON and CSV outputs to be excluded.'

    $csvHeader = (Get-Content -LiteralPath $csvPath -TotalCount 1).TrimStart([char]0xFEFF)
    Assert-True ($csvHeader -eq '"relativePath","extension","sizeBytes","sha256","probableCategory","modifiedTime"') 'CSV header order or names changed.'

    $jsonText = Get-Content -LiteralPath $jsonPath -Raw
    Assert-True (-not $jsonText.Contains('SYNTHETIC_SECRET')) 'Manifest must not contain .env contents.'
    Assert-True (-not $jsonText.Contains('service-token.json')) 'Sensitive filenames must not be inventoried.'

    $jsonBeforeSecondRun = [Convert]::ToBase64String([System.IO.File]::ReadAllBytes($jsonPath))
    $csvBeforeSecondRun = [Convert]::ToBase64String([System.IO.File]::ReadAllBytes($csvPath))
    $secondRunFailed = $false
    $secondRunError = ''
    try {
        & $toolPath `
            -ProjectRoot $fixtureRoot `
            -OutputPath $jsonPath `
            -ExcludeBackupPatterns @('*.bak', '*.custombak') `
            -ExcludeLogPatterns @('*.log', '*.trace')
    }
    catch {
        $secondRunFailed = $true
        $secondRunError = $_.Exception.Message
    }

    Assert-True $secondRunFailed 'Expected a second run with existing outputs to fail.'
    Assert-True ($secondRunError -like '*Refusing to overwrite existing JSON output*') 'Expected the repeat run to fail because its JSON output already exists.'
    $jsonAfterSecondRun = [Convert]::ToBase64String([System.IO.File]::ReadAllBytes($jsonPath))
    $csvAfterSecondRun = [Convert]::ToBase64String([System.IO.File]::ReadAllBytes($csvPath))
    Assert-True ($jsonAfterSecondRun -eq $jsonBeforeSecondRun) 'Existing JSON output changed after refused repeat run.'
    Assert-True ($csvAfterSecondRun -eq $csvBeforeSecondRun) 'Existing CSV output changed after refused repeat run.'

    Write-Host 'PASS: asset manifest synthetic fixture checks.'
}
finally {
    if (Test-Path -LiteralPath $fixtureRoot) {
        Remove-Item -LiteralPath $fixtureRoot -Recurse -Force
    }
}
