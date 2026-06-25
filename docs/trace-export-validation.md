# Trace Export Validation and Diagnostics

The `glassbox debug` command includes comprehensive validation and diagnostic capabilities for trace export operations. This document describes the validation checks, error handling, and troubleshooting guidance.

---

## Overview

Trace export validation occurs at multiple stages:

1. **Pre-flight validation** — CLI flag validation before any simulation
2. **Pre-export validation** — Trace data and configuration validation before export
3. **Format compatibility** — Format-specific checks for data compatibility
4. **Export execution** — File system and I/O validation during write
5. **Error resilience** — Retry logic, atomic writes, and recovery mechanisms
6. **Version compatibility** — Forward/backward compatibility and migration support

---

## Resilient Export Features

### Automatic Error Recovery

Trace exports now support resilient export mode with automatic recovery from transient failures:

```bash
# Export with resilience features (enabled by default in CLI)
glassbox debug --trace-output ./output/trace.json --network testnet <tx-hash>
```

**Resilience features:**
- **Retry logic** - Automatic retry for transient errors (network issues, temporary file locks)
- **Atomic writes** - Uses atomic write-rename to prevent partial file corruption
- **Checksum verification** - Computes SHA-256 checksum for integrity verification
- **Metadata files** - Writes companion .meta.json with export metadata
- **Backup creation** - Optional backup of existing files before overwrite
- **Trace sanitization** - Automatic repair of common trace corruption issues

###Configuration Options

Resilient export can be configured programmatically:

```go
recoveryOpts := trace.ExportRecoveryOptions{
    EnableChecksum: true,          // Compute checksums for verification
    EnableMetadata: true,           // Write .meta.json companion files
    AtomicWrite:    true,           // Use atomic write-rename
    BackupExisting: false,          // Backup files before overwrite
    MaxRetries:     3,              // Number of retry attempts
    RetryDelay:     100 * time.Millisecond,
}

err := trace.ExportWithResilience(executionTrace, "json", "./output.json", opts, recoveryOpts)
```

---

## Version Compatibility

### Versioned Trace Exports

JSON exports now include version information for compatibility tracking:

```json
{
  "version": {
    "major": 1,
    "minor": 0,
    "patch": 0
  },
  "trace": {
    "transaction_hash": "abc123...",
    "states": [...]
  }
}
```

### Compatibility Rules

**Major version compatibility:**
- Major version changes indicate breaking changes
- Traces from different major versions are incompatible
- Example: v1.x.x cannot load v2.x.x traces

**Minor version compatibility:**
- Newer CLI can read older minor versions (backward compatible)
- Older CLI cannot read newer minor versions (unless AllowNewerMinor is enabled)
- Example: v1.2.x can read v1.0.x traces

**Patch version compatibility:**
- Fully compatible across all patch versions
- Bug fixes and non-breaking improvements only

### Loading Traces with Compatibility Checks

```go
compatOpts := trace.CompatibilityOptions{
    StrictVersionCheck: false,   // Allow minor version differences
    AllowNewerMinor:    true,    // Load traces from newer minor versions
    AllowDowngrade:     false,   // Disallow exporting to older versions
    PreserveLegacyFields: true,  // Keep deprecated fields during migration
}

trace, err := trace.LoadVersionedTrace("./trace.json", compatOpts)
```

**Example validation output:**
```
Info: migrated trace from version 1.0.0 to 1.1.0
Warning: loaded legacy trace format (no version info)
  Consider re-exporting with current version for full compatibility
```

---

## Trace Sanitization and Recovery

### Automatic Sanitization

Traces are automatically sanitized before export to handle common corruption issues:

**Fixed issues:**
- Missing or zero timestamps → Interpolated from start/end times
- Missing transaction hash → Set to placeholder
- Step index mismatches → Corrected to match array position
- Missing operation/event types → Set to placeholder
- Overly long error messages (>10KB) → Truncated with marker

**Example:**
```go
sanitized, errs := trace.SanitizeTrace(corruptedTrace)
// Returns sanitized trace + list of repairs made

for _, err := range errs {
    fmt.Printf("Repaired: %v\n", err)
}
// Output:
// Repaired: step 5 has incorrect index 10, corrected
// Repaired: step 7 missing timestamp, interpolated
```

### Manual Recovery

Recover traces from potentially corrupted export files:

```bash
# Verify trace integrity
glassbox trace verify ./trace.json

# Output:
# [OK] Checksum verification passed
# [OK] Format validation passed
# [OK] Structure validation passed
```

**Programmatic recovery:**
```go
trace, errs := trace.RecoverTrace("./corrupted-trace.json")
if len(errs) > 0 {
    for _, err := range errs {
        fmt.Printf("Recovery warning: %v\n", err)
    }
}
// Trace is usable even if warnings exist
```

---

## Format Conversion

### Converting Between Formats

Convert traces from JSON to other formats:

```bash
# Convert JSON to HTML
glassbox trace convert --input ./trace.json --output ./trace.html --format html

# Convert JSON to Markdown
glassbox trace convert --input ./trace.json --output ./trace.md --format markdown
```

**Limitations:**
- Only JSON format can be converted back to trace objects
- HTML, Markdown, and Text are presentation-only (one-way conversion)

**Programmatic conversion:**
```go
err := trace.ConvertFormat("./input.json", "./output.html", "html")
// Output: Successfully converted trace from json to html
```

---

## Integrity Verification

### Checksum Verification

Verify trace file integrity using checksums:

```go
err := trace.VerifyExport("./trace.json")
if err != nil {
    // Checksum mismatch or corruption detected
}
```

**Error examples:**
```
checksum mismatch
  Expected: a1b2c3...
  Actual:   d4e5f6...
  The trace file may have been modified or corrupted
  Fix: re-export the trace with glassbox debug --trace-output
```

### Metadata Validation

Companion .meta.json files contain export metadata:

```json
{
  "version": "1.0",
  "format": "json",
  "transaction_hash": "abc123...",
  "exported_at": "2026-01-15T10:30:00Z",
  "step_count": 150,
  "checksum": "a1b2c3d4...",
  "cli_version": "1.2.3",
  "hostname": "ci-server-01"
}
```

---

## Pre-flight Validation (CLI Flags)

The debug command validates trace-related flags in `PreRunE` before any network or simulator operations:

### `--trace-verbosity`

**Valid values:** `summary`, `normal`, `verbose`

**Validation:**
- Must be one of the three supported values (case-insensitive)
- Checked at parse time before any execution

**Error example:**
```
invalid --trace-verbosity "ultra" — must be one of: summary, normal, verbose
  Fix: use --trace-verbosity normal (default), summary (minimal), or verbose (detailed)
```

### `--format`

**Valid values:** `text`, `json`, `html`, `markdown` (or `md`)

**Validation:**
- Must be one of the supported export formats
- Checked before simulation begins

**Error example:**
```
invalid trace export format "yaml" — must be one of: text, json, html, markdown
  Fix: use --format html (interactive), json (machine-readable), markdown (shareable), or text (CLI output)
```

### `--trace-output`

**Validation:**
- Must be a file path, not a directory path (no trailing `/` or `\`)
- Cannot contain null bytes
- Path traversal sequences (`..`) trigger a security warning
- Parent directory must exist or be creatable

**Error examples:**
```
--trace-output "./traces/" looks like a directory path; provide a full file path
  Fix: specify a complete file path (e.g. ./traces/trace.html or ./output/trace.json)
  Example: glassbox debug --trace-output ./traces/debug-$(date +%Y%m%d).html <tx-hash>
```

```
--trace-output "../../../etc/passwd" contains directory traversal sequences (..)
  Fix: use absolute paths or relative paths without '..' for security
  Example: use './output/trace.html' instead of '../output/trace.html'
```

---

## Pre-export Validation

Before attempting to write a trace export, the system validates all export parameters:

### Trace Data Validation

**Checks:**
- Trace object is not nil
- Trace contains at least one execution state
- All state step indices match their position in the array
- Event types are recognized (unrecognized types trigger warnings)

**Error example:**
```
execution trace for transaction "5c0a1234..." contains no steps — trace export would be empty
  Possible causes:
    - Simulation did not produce any diagnostic events
    - Transaction envelope is invalid
    - Simulator version is incompatible
  Fix: verify the transaction executed successfully
  Recommended: run 'glassbox doctor' to check simulator compatibility
```

### Format and Path Validation

**Checks:**
- Export format is not empty and is supported
- Output path is not empty
- Output path is not a directory
- Output path does not contain invalid characters

**Error example:**
```
export format is empty — must specify one of: html, markdown, json, text
  Fix: provide --format html (default), markdown, json, or text
```

### Export Options Validation

**Checks:**
- Comment count does not exceed 100
- Individual comment length does not exceed 10,000 characters
- Session metadata keys and values are valid strings

**Error example:**
```
too many comments (150) — maximum is 100 comments per trace export
  Fix: reduce the number of comments or split into multiple exports
```

---

## Format Compatibility Checks

Each export format has specific compatibility requirements:

### JSON Format

**Requirements:**
- All trace data must be JSON-serializable
- No circular references
- Step indices must be sequential and match array position

**Error example:**
```
trace step mismatch at position 5: expected step 5 but got 10 — trace may be corrupted
```

### HTML Format

**Requirements:**
- Argument strings should not exceed 50,000 characters (browser rendering limit)
- Special characters are automatically HTML-escaped

**Error example:**
```
step 3 has very large arguments (75000 chars) that may cause browser rendering issues in HTML format — consider using JSON format instead
```

### Markdown Format

**Requirements:**
- Works with most data
- Very long lines may require viewer adjustments

### Text Format

**Requirements:**
- Most permissive format
- No special constraints

---

## Export Execution Errors

Errors that occur during file write operations include detailed remediation:

### Directory Creation Failure

```
failed to create trace export directory: permission denied
  Directory: /restricted/traces
  Fix: ensure you have write permissions to the parent directory
  Or choose a different output path with --trace-output
```

### File Write Failure

```
failed to write trace export file: no space left on device
  Path: /tmp/trace.html
  Fix: ensure you have write permissions and sufficient disk space
  Check: ls -la /tmp
```

### Template Rendering Failure

```
failed to generate HTML trace: template execution error: ...
  This may indicate invalid trace data or a template rendering error
  Check that all trace fields are properly populated
```

---

## Validation in Dry-Run Mode

When using `--dry-run`, trace output configuration is validated without executing the simulation:

```sh
glassbox debug --dry-run --trace-output ./invalid/ --network testnet <tx-hash>
```

**Output:**
```
Additional environment checks:
[FAIL] Trace output validation failed: --trace-output "./invalid/" looks like a directory path
       Fix: ensure trace output path is valid and format is correct
```

---

## Multiple Validation Errors

When multiple validation errors are detected, all failures are reported together so they can be fixed in a single pass:

```
3 trace input validation error(s):
  1. invalid --trace-verbosity "ultra" — must be one of: summary, normal, verbose
     Fix: use --trace-verbosity normal (default), summary (minimal), or verbose (detailed)
  2. invalid trace export format "yaml" — must be one of: text, json, html, markdown
     Fix: use --format html (interactive), json (machine-readable), markdown (shareable), or text (CLI output)
  3. --trace-output "./traces/" looks like a directory path; provide a full file path
     Fix: specify a complete file path (e.g. ./traces/trace.html or ./output/trace.json)
```

---

## Best Practices

### 1. Use Dry-Run for Validation

Always run with `--dry-run` first when setting up trace export in CI/CD:

```sh
glassbox debug --dry-run \
  --network testnet \
  --trace-output ./artifacts/trace.html \
  --format html \
  <tx-hash>
```

### 2. Choose the Right Format

- **HTML**: Interactive viewing in browsers, best for manual analysis
- **JSON**: Machine-readable, best for CI/CD and automated processing
- **Markdown**: Shareable in chat/issues, best for collaboration
- **Text**: Plain CLI output, best for simple logging

### 3. Organize Output Paths

Use dated directories for trace exports:

```sh
glassbox debug \
  --trace-output "./traces/$(date +%Y-%m-%d)/${TX_HASH}.html" \
  --format html \
  $TX_HASH
```

### 4. Validate Before Large Exports

For traces with many steps, validate format compatibility first:

```sh
# Check trace size first
glassbox debug --format json $TX_HASH | jq '.States | length'

# Use JSON for very large traces (>1000 steps)
if [ $STEPS -gt 1000 ]; then
  glassbox debug --format json --trace-output ./trace.json $TX_HASH
else
  glassbox debug --format html --trace-output ./trace.html $TX_HASH
fi
```

---

## Troubleshooting

### "Trace contains no steps"

**Cause:** The simulator did not produce any diagnostic events.

**Solutions:**
1. Verify the transaction hash is correct
2. Run `glassbox doctor` to check simulator compatibility
3. Check that the transaction actually executed on the network
4. Ensure the simulator binary is up-to-date

### "Trace step mismatch"

**Cause:** The trace data structure is corrupted.

**Solutions:**
1. Re-run the debug command to regenerate the trace
2. Try recovering with `trace.RecoverTrace()` or automatic sanitization
3. Check for filesystem corruption if using `--save-snapshots`
4. Verify the simulator version matches the CLI version

### "Very large arguments"

**Cause:** Contract arguments exceed browser rendering limits for HTML export.

**Solutions:**
1. Use JSON format instead: `--format json`
2. Filter the trace to specific event types
3. Use `--trace-verbosity summary` for less detail

### "Permission denied"

**Cause:** Insufficient write permissions to the output directory.

**Solutions:**
1. Choose a different output path with write permissions
2. Create the output directory manually with correct permissions
3. Check filesystem mount options (read-only mounts)

### "Checksum mismatch"

**Cause:** Trace file was modified after export or became corrupted.

**Solutions:**
1. Re-export the trace: `glassbox debug --trace-output ./trace.json <tx-hash>`
2. Check disk health if corruption is frequent
3. Verify no other process is modifying the file
4. Use atomic write mode (enabled by default)

### "Version incompatible"

**Cause:** Trying to load a trace from incompatible CLI version.

**Solutions:**
1. Upgrade CLI to match trace version: `glassbox version`
2. Re-export trace with current CLI version
3. Enable `AllowNewerMinor` for minor version differences
4. Check migration compatibility in documentation

### "Resource temporarily unavailable"

**Cause:** Transient file system or network error.

**Solutions:**
1. Retry the export operation (automatic with resilient export)
2. Check disk space with `df -h`
3. Verify no other process has files locked
4. Increase retry count in `ExportRecoveryOptions`

### "Format detection failed"

**Cause:** File doesn't match any known trace format.

**Solutions:**
1. Verify file is not corrupted: `file ./trace.json`
2. Check file was created by glassbox: `head -20 ./trace.json`
3. Try specifying format explicitly: `--format json`
4. Use JSON validator: `jq . ./trace.json`

---

## See Also

- [Debug Command Reference](./debug-command.md)
- [Trace Export Annotations](./trace-export-annotations.md)
- [Event Schemas](./event-schemas.md)
- [JSON Output Format](./json-output.md)
