[CmdletBinding()]
param(
    [Parameter(Mandatory = $true)]
    [ValidateNotNullOrEmpty()]
    [string] $ProjectRoot,

    [Parameter(Mandatory = $true)]
    [ValidateNotNullOrEmpty()]
    [string] $OutputPath,

    [string[]] $ExcludeBackupPatterns = @(
        '*.bak', '*.backup', '*.old', '*.orig', '*.swp', '*.tmp', '*~'
    ),

    [string[]] $ExcludeLogPatterns = @(
        '*.log', '*.log.*'
    )
)

Set-StrictMode -Version 2.0
$ErrorActionPreference = 'Stop'

function Add-InventoryWarning {
    param(
        [string] $Path,
        [string] $Message
    )

    $warning = [pscustomobject][ordered]@{
        path    = $Path
        message = $Message
    }
    $script:Warnings.Add($warning)
    Write-Warning ($Message + ': ' + $Path)
}

function Test-PatternMatch {
    param(
        [string] $RelativePath,
        [string] $FileName,
        [string[]] $Patterns
    )

    foreach ($pattern in $Patterns) {
        if ([string]::IsNullOrWhiteSpace($pattern)) {
            continue
        }
        if (($FileName -like $pattern) -or ($RelativePath -like $pattern)) {
            return $true
        }
    }
    return $false
}

function Test-ExcludedFileName {
    param([string] $FileName)

    if ($FileName.StartsWith('.env', [System.StringComparison]::OrdinalIgnoreCase)) {
        return $true
    }

    if ($FileName -match '(?i)(^|[._-])(credential|credentials|token|tokens|password|passwd|secret|secrets|private[ -_.]?key|api[ -_.]?key|access[ -_.]?key|auth[ -_.]?key|signing[ -_.]?key|id_rsa|id_ed25519)([._-]|$)') {
        return $true
    }

    if ($FileName -match '(?i)\.(pem|p12|pfx|key)$') {
        return $true
    }

    return $false
}

function Get-ProbableCategory {
    param([string] $Extension)

    switch -Regex ($Extension.ToLowerInvariant()) {
        '^\.(png|jpe?g|gif|bmp|tiff?|webp|svg|dds|tga|exr)$' { return 'image' }
        '^\.(wav|mp3|ogg|flac|aiff?|m4a)$' { return 'audio' }
        '^\.(mp4|mov|avi|mkv|webm|ogv)$' { return 'video' }
        '^\.(ttf|otf|woff2?)$' { return 'font' }
        '^\.(glb|gltf|fbx|obj|dae|blend|stl)$' { return 'model' }
        '^\.(tscn|tres|scn|scene|prefab)$' { return 'scene' }
        '^\.(gd|cs|py|js|ts|go|ps1|sh)$' { return 'script' }
        '^\.(ini|cfg|conf|toml|yaml|yml|env|godot)$' { return 'config' }
        '^\.(json|xml|csv|tsv|db|sqlite|res|resource)$' { return 'data' }
        '^\.(zip|7z|rar|tar|gz|bz2)$' { return 'archive' }
        '^\.(md|txt|pdf|docx?|xlsx?)$' { return 'document' }
        default { return 'other' }
    }
}

$resolvedRoot = Resolve-Path -LiteralPath $ProjectRoot
if ($resolvedRoot.Provider.Name -ne 'FileSystem') {
    throw 'ProjectRoot must resolve to a file-system directory.'
}
$rootPath = [System.IO.Path]::GetFullPath($resolvedRoot.ProviderPath)
$volumeRoot = [System.IO.Path]::GetPathRoot($rootPath)
if (-not [string]::Equals($rootPath, $volumeRoot, [System.StringComparison]::OrdinalIgnoreCase)) {
    $rootPath = $rootPath.TrimEnd([char[]]@(
        [System.IO.Path]::DirectorySeparatorChar,
        [System.IO.Path]::AltDirectorySeparatorChar
    ))
}
if (-not [System.IO.Directory]::Exists($rootPath)) {
    throw 'ProjectRoot must be an existing directory.'
}

$outputProviderPath = $ExecutionContext.SessionState.Path.GetUnresolvedProviderPathFromPSPath($OutputPath)
$jsonPath = [System.IO.Path]::GetFullPath($outputProviderPath)
$csvPath = [System.IO.Path]::ChangeExtension($jsonPath, '.csv')
if ([string]::Equals($jsonPath, $csvPath, [System.StringComparison]::OrdinalIgnoreCase)) {
    throw 'OutputPath must identify the JSON manifest, not a .csv file.'
}
if ([System.IO.File]::Exists($jsonPath) -or [System.IO.Directory]::Exists($jsonPath)) {
    throw ('Refusing to overwrite existing JSON output: ' + $jsonPath)
}
if ([System.IO.File]::Exists($csvPath) -or [System.IO.Directory]::Exists($csvPath)) {
    throw ('Refusing to overwrite existing CSV output: ' + $csvPath)
}
$excludedOutputPaths = @($jsonPath, $csvPath)

$excludedDirectoryNames = @(
    '.git', '.svn', '.hg', 'node_modules', 'bower_components', 'vendor',
    'packages', 'dependencies', 'deps', 'third_party', 'third-party',
    'submodules', 'pods', '.venv', 'venv', 'site-packages', '__pycache__',
    '.cache', 'cache', '.npm', '.yarn', '.pnpm-store', '.pytest_cache',
    '.mypy_cache', '.ruff_cache', '.parcel-cache', 'tmp', 'temp', '.temp',
    'build', '.build', 'dist', 'out', 'bin', 'obj', '.godot', 'target',
    'coverage'
)
$script:Warnings = New-Object 'System.Collections.Generic.List[object]'
$assets = New-Object 'System.Collections.Generic.List[object]'
$directories = New-Object 'System.Collections.Generic.Queue[System.IO.DirectoryInfo]'

$rootInfo = New-Object System.IO.DirectoryInfo($rootPath)
try {
    if (($rootInfo.Attributes -band [System.IO.FileAttributes]::ReparsePoint) -ne 0) {
        Add-InventoryWarning -Path '.' -Message 'Project root is a reparse point and was not traversed'
    }
    else {
        $directories.Enqueue($rootInfo)
    }
}
catch {
    Add-InventoryWarning -Path '.' -Message ('Could not inspect project root (' + $_.Exception.Message + ')')
}

while ($directories.Count -gt 0) {
    $directory = $directories.Dequeue()
    try {
        $entries = $directory.GetFileSystemInfos()
    }
    catch {
        $relativeDirectory = $directory.FullName.Substring($rootPath.Length).TrimStart([char[]]@(
            [System.IO.Path]::DirectorySeparatorChar,
            [System.IO.Path]::AltDirectorySeparatorChar
        )).Replace('\', '/')
        if ([string]::IsNullOrEmpty($relativeDirectory)) {
            $relativeDirectory = '.'
        }
        Add-InventoryWarning -Path $relativeDirectory -Message ('Could not enumerate directory (' + $_.Exception.Message + ')')
        continue
    }

    foreach ($entry in $entries) {
        $relativePath = $entry.FullName.Substring($rootPath.Length).TrimStart([char[]]@(
            [System.IO.Path]::DirectorySeparatorChar,
            [System.IO.Path]::AltDirectorySeparatorChar
        )).Replace('\', '/')

        try {
            $attributes = $entry.Attributes
        }
        catch {
            Add-InventoryWarning -Path $relativePath -Message ('Could not inspect file-system entry (' + $_.Exception.Message + ')')
            continue
        }

        if (($attributes -band [System.IO.FileAttributes]::ReparsePoint) -ne 0) {
            continue
        }

        if (($attributes -band [System.IO.FileAttributes]::Directory) -ne 0) {
            if ($entry.Name.StartsWith('.env', [System.StringComparison]::OrdinalIgnoreCase) -or
                ($excludedDirectoryNames -contains $entry.Name.ToLowerInvariant())) {
                continue
            }
            $directories.Enqueue([System.IO.DirectoryInfo]$entry)
            continue
        }

        if (Test-ExcludedFileName -FileName $entry.Name) {
            continue
        }
        if (Test-PatternMatch -RelativePath $relativePath -FileName $entry.Name -Patterns $ExcludeBackupPatterns) {
            continue
        }
        if (Test-PatternMatch -RelativePath $relativePath -FileName $entry.Name -Patterns $ExcludeLogPatterns) {
            continue
        }

        $isOutput = $false
        foreach ($manifestPath in $excludedOutputPaths) {
            if ([string]::Equals($entry.FullName, $manifestPath, [System.StringComparison]::OrdinalIgnoreCase)) {
                $isOutput = $true
                break
            }
        }
        if ($isOutput) {
            continue
        }

        try {
            $file = [System.IO.FileInfo]$entry
            $hash = Get-FileHash -LiteralPath $file.FullName -Algorithm SHA256
            $assets.Add([pscustomobject][ordered]@{
                relativePath     = $relativePath
                extension        = $file.Extension
                sizeBytes        = [long]$file.Length
                sha256           = $hash.Hash.ToLowerInvariant()
                probableCategory = Get-ProbableCategory -Extension $file.Extension
                modifiedTime     = $file.LastWriteTimeUtc.ToString('o')
            })
        }
        catch {
            Add-InventoryWarning -Path $relativePath -Message ('Could not read file metadata or calculate SHA-256 (' + $_.Exception.Message + ')')
        }
    }
}

$outputDirectory = [System.IO.Path]::GetDirectoryName($jsonPath)
if (-not [string]::IsNullOrEmpty($outputDirectory) -and -not [System.IO.Directory]::Exists($outputDirectory)) {
    [void][System.IO.Directory]::CreateDirectory($outputDirectory)
}

$manifest = [ordered]@{
    generatedAtUtc = [DateTime]::UtcNow.ToString('o')
    assets         = @($assets.ToArray())
    warnings       = @($script:Warnings.ToArray())
}
$json = ConvertTo-Json -InputObject $manifest -Depth 5
Set-Content -LiteralPath $jsonPath -Value $json -Encoding UTF8

$csvHeaders = @('relativePath', 'extension', 'sizeBytes', 'sha256', 'probableCategory', 'modifiedTime')
$csvRows = @($assets.ToArray() | Select-Object -Property $csvHeaders)
if ($csvRows.Count -eq 0) {
    Set-Content -LiteralPath $csvPath -Value ('"' + ($csvHeaders -join '","') + '"') -Encoding UTF8
}
else {
    $csvRows | Export-Csv -LiteralPath $csvPath -NoTypeInformation -Encoding UTF8
}

Write-Host ('Wrote {0} assets to {1} and {2}.' -f $assets.Count, $jsonPath, $csvPath)
if ($script:Warnings.Count -gt 0) {
    Write-Warning ('Inventory completed with {0} warning(s); see the warnings array in the JSON manifest.' -f $script:Warnings.Count)
}
