# Release Pipeline Design

**Date:** 2026-03-01

## Goal

Automate cross-platform release builds for ralph-loop-go so users can easily install pre-built binaries on Intel Mac, M-series Mac, Windows, and Linux.

## Approach: GoReleaser

GoReleaser handles cross-compilation, archiving, checksums, and GitHub Release creation from a single config file. This is the standard toolchain for Go project releases.

## Components

### 1. `.goreleaser.yaml`

Build targets:
- `linux/amd64`
- `darwin/amd64` (Intel Mac)
- `darwin/arm64` (M-series Mac)
- `windows/amd64`

Archives: `.tar.gz` for unix, `.zip` for windows. Generates `checksums.txt` with SHA256 hashes. Binary name: `ralph-loop-go`.

Uses vendored dependencies (`-mod=vendor`).

### 2. `.github/workflows/release.yml`

Triggers:
- Tag push matching `v*` (e.g., `git tag v1.0.0 && git push --tags`)
- Manual dispatch from Actions tab (with optional tag input)

Steps: checkout -> setup Go -> run GoReleaser -> creates GitHub Release with all artifacts.

### 3. `install.sh`

Curl-pipe-bash installer at the repo root.

Behavior:
- Detects OS (`linux`/`darwin`) and arch (`amd64`/`arm64`)
- Downloads the correct archive from the latest GitHub Release
- Extracts binary to `/usr/local/bin` (falls back to `~/.local/bin` if no sudo)
- Verifies SHA256 checksum after download

Usage:
```bash
curl -sSfL https://raw.githubusercontent.com/fireynis/ralph-loop-tui/main/install.sh | bash
```

## Release Artifacts

For a release like `v1.0.0`, the GitHub Release will contain:
- `ralph-loop-go_1.0.0_linux_amd64.tar.gz`
- `ralph-loop-go_1.0.0_darwin_amd64.tar.gz`
- `ralph-loop-go_1.0.0_darwin_arm64.tar.gz`
- `ralph-loop-go_1.0.0_windows_amd64.zip`
- `checksums.txt`
