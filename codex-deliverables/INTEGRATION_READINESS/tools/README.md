# Local asset inventory

`Generate-WYDAssetManifest.ps1` creates a read-only inventory of files under a
project directory. It does not copy source files or send data anywhere. The
output contains each included file's project-relative path, extension, byte
size, SHA-256, probable category, and UTC modified time. JSON also includes
explicit warnings for paths that could not be inspected.

The script uses only Windows PowerShell built-ins and is intended for Windows
PowerShell 5.1. `OutputPath` names the JSON file; a CSV with the same base name
is written beside it. Relative output paths are resolved from the current
working directory.

For safety, both the JSON output and paired CSV output must have paths that do
not already exist. The script stops before scanning or writing if either path
is present; it has no overwrite or force option. Choose a new output name for
each run.

```powershell
.\Generate-WYDAssetManifest.ps1 `
  -ProjectRoot 'D:\Work\GameProject' `
  -OutputPath 'D:\Reports\WYDAssetManifest.json'
```

By default, common backup (`*.bak`, `*.backup`, `*.old`, `*.orig`, `*.swp`,
`*.tmp`, `*~`) and log (`*.log`, `*.log.*`) patterns are excluded. Pass
replacement pattern lists when a project uses different conventions:

```powershell
.\Generate-WYDAssetManifest.ps1 `
  -ProjectRoot 'D:\Work\GameProject' `
  -OutputPath '.\reports\manifest.json' `
  -ExcludeBackupPatterns @('*.backup', '*.snapshot') `
  -ExcludeLogPatterns @('*.log', '*.trace')
```

The tool also excludes source-control metadata, common dependency/build/cache/
temporary directories, `.env*` files, and filenames that appear to contain
credentials, tokens, passwords, secrets, or private-key material. Reparse
points (including symlinks) are skipped rather than followed. The JSON and CSV
outputs are excluded from inventory when they are inside `ProjectRoot`.

The CSV columns are always, in order:
`relativePath, extension, sizeBytes, sha256, probableCategory, modifiedTime`.
Skipped sensitive files are not opened or logged. Warnings include paths and
error descriptions only; file contents are never included.

Example output (synthetic format illustration only):

```json
{
  "generatedAtUtc": "2026-11-06T00:00:00.0000000Z",
  "assets": [
    {
      "relativePath": "Data/DEMONSTRACAO-sprite.png",
      "extension": ".png",
      "sizeBytes": 1234,
      "sha256": "0000000000000000000000000000000000000000000000000000000000000000",
      "probableCategory": "image",
      "modifiedTime": "2026-11-06T00:00:00.0000000Z"
    }
  ],
  "warnings": []
}
```

```csv
"relativePath","extension","sizeBytes","sha256","probableCategory","modifiedTime"
"Data/DEMONSTRACAO-sprite.png",".png","1234","0000000000000000000000000000000000000000000000000000000000000000","image","2026-11-06T00:00:00.0000000Z"
```

## Synthetic test

Run the standalone test with Windows PowerShell 5.1:

```powershell
.\tests\Test-Generate-WYDAssetManifest.ps1
```

The test creates and removes synthetic files under its own tools directory. It
checks successful fresh output creation and verifies that a repeat run fails
without changing either existing output. It does not require or inspect game
assets.
