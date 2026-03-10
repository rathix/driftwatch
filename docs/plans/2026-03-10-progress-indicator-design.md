# Progress Indicator Design

**Date:** 2026-03-10
**Status:** Approved

## Goal

Show real-time progress feedback during `driftwatch scan` via a spinner with stage messages on stderr.

## Design

Lightweight spinner (no external deps) using braille characters `⠋⠙⠹⠸⠼⠴⠦⠧⠇⠏`. Single file `pkg/progress/spinner.go`.

### API

```go
spinner := progress.NewSpinner(os.Stderr)  // returns no-op if not TTY
spinner.Update("Discovering sources...")
spinner.Update("Scanning infrastructure/ (2/5)...")
spinner.Stop()  // clears spinner line
```

### Stage Messages

- `Discovering sources...`
- `Connecting to cluster...`
- `Scanning <path> (<n>/<total>)...`
- `Enriching with Flux status...`
- `Detecting extras...`

### Behavior

- Writes to stderr only (stdout stays clean for JSON/piped output)
- Auto-disabled when stderr is not a TTY or `--output json`
- Uses `\r` to overwrite the current line
- Clears line on Stop() before report prints
- Goroutine with 100ms ticker cycles spinner frames
