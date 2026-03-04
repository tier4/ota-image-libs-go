# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Project Overview

`ota-image-libs-go` is a zero-dependency Go library providing streaming read support for OTA image version 1 artifacts.
It is the Go implementation of the artifact reader from `ota-image-libs` (Python), covering only the read/streaming portion.

An OTA image v1 artifact is a strict subset of ZIP archive where all entries are stored uncompressed with fixed permission bits and datetime.
The library exposes a `tar`-like streaming interface (`Next`/`Read` pattern) for sequential IO streams.

## Commands for Dev

**Testing:**

```bash
go test -v ./artifact                                    # Run all tests
go test -v -run TestReadOTAImageArtifact ./artifact      # Run a specific test
go test -coverprofile=.test/coverage.out ./artifact      # With coverage
```

The large file test (`TestReadOTAImageArtifactWith6Gblob`) requires decompressing test data first:

```bash
zstd -d ./artifact/testdata/ota_image_6g_blob.zip.zst -o ./artifact/testdata/ota_image_6g_blob.zip
```

**Linting & Formatting (golangci-lint v2):**

```bash
golangci-lint run ./...          # Lint
golangci-lint fmt ./...          # Format (goimports)
```

**Pre-commit:**

```bash
pre-commit install               # Install hooks (once after cloning)
pre-commit run                   # Run on changed files only
pre-commit run --all-files       # Run all hooks manually
```

Hooks run on every commit:

- fix-byte-order-marker
- check-merge-conflict
- check-yaml
- end-of-file-fixer
- trailing-whitespace
- mixed-line-ending
- markdownlint (with auto-fix, config in `.markdownlint.yaml`)
- golangci-lint
- golangci-lint-fmt

## Architecture

### Package Layout

The repository contains a single package:

- **`artifact/`** — Streaming reader for OTA image v1 artifacts.

### Key Types and API (`artifact/`)

| File | Purpose |
| --- | --- |
| `struct.go` | `LocalFileHeader` struct (Name, Size, CRC32) with ZIP format constants |
| `common.go` | `readBuf` helper type for little-endian binary parsing |
| `reader.go` | `StreamReader` — core streaming reader with `Next()` and `Read()` API |

**Public API:**

- `NewReader(r io.Reader) *StreamReader` — Create a new streaming reader.
- `(*StreamReader).Next() (*LocalFileHeader, error)` — Advance to the next file entry.
  Returns header or `io.EOF` if reaching the end of the archive.
- `(*StreamReader).Read(b []byte) (int, error)` — Read data from the current file entry.
- `(*LocalFileHeader).IsDir() bool` — Check if the entry is a directory.

**Internal:**

- `checksumFileStreamReader` — Wraps file data reading with CRC32 validation on EOF.

### Dependencies

Zero external dependencies.
Uses only the Go standard library (`archive/zip`, `encoding/binary`, `hash/crc32`, `io`, `errors`).

## CI/CD

**`.github/workflows/test.yaml` — Test CI**

- Triggers on PRs to `main`, pushes to `main` (when `.go`/`go.mod`/`go.sum` files change), and manual dispatch.
- Runs `go test` with coverage on `ubuntu-latest` (timeout: 3 minutes).
- Decompresses `ota_image_6g_blob.zip.zst` for the large file test.
  Uploads coverage to SonarCloud.

## Code Style

- **Linters** (golangci-lint v2, config in `.golangci.yml`): bodyclose, errcheck, gosec, govet, ineffassign, misspell, prealloc, staticcheck, unconvert, unused
- **Formatter**: goimports
- **Go version**: 1.24 (toolchain go1.24.12)
- **Quality gates**: SonarCloud (coverage, security rating, maintainability rating)
