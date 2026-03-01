# Release Pipeline Implementation Plan

> **For Claude:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task.

**Goal:** Set up GoReleaser-based GitHub Actions to build and publish cross-platform binaries, plus a curl-pipe-bash install script.

**Architecture:** GoReleaser handles cross-compilation, archiving, and GitHub Release creation from `.goreleaser.yaml`. A GitHub Actions workflow triggers on tag push or manual dispatch. An install script at the repo root auto-detects platform and downloads the right binary.

**Tech Stack:** GoReleaser v2, GitHub Actions, POSIX shell

---

### Task 1: Create `.goreleaser.yaml`

**Files:**
- Create: `.goreleaser.yaml`

**Step 1: Create the GoReleaser config**

```yaml
version: 2

builds:
  - binary: ralph-loop-go
    env:
      - CGO_ENABLED=0
    flags:
      - -trimpath
      - -mod=vendor
    ldflags:
      - -s -w
    goos:
      - linux
      - darwin
      - windows
    goarch:
      - amd64
      - arm64
    ignore:
      - goos: windows
        goarch: arm64

archives:
  - name_template: "{{ .ProjectName }}_{{ .Version }}_{{ .Os }}_{{ .Arch }}"
    formats:
      - tar.gz
    format_overrides:
      - goos: windows
        formats:
          - zip

checksum:
  name_template: "checksums.txt"
  algorithm: sha256

changelog:
  sort: asc
  filters:
    exclude:
      - "^docs:"
      - "^test:"
      - "^ci:"

release:
  github:
    owner: fireynis
    name: ralph-loop-tui
  draft: false
  name_template: "v{{.Version}}"
```

**Step 2: Add `dist/` to `.gitignore`**

GoReleaser outputs to `dist/` by default. Add it to `.gitignore`:

```
dist/
```

**Step 3: Verify config locally (optional)**

Run: `goreleaser check` (if goreleaser is installed locally)
Expected: `config is valid`

If not installed, skip — CI will validate.

**Step 4: Commit**

```bash
git add .goreleaser.yaml .gitignore
git commit -m "feat: add GoReleaser config for cross-platform builds"
```

---

### Task 2: Create GitHub Actions release workflow

**Files:**
- Create: `.github/workflows/release.yml`

**Step 1: Create the workflow directory**

```bash
mkdir -p .github/workflows
```

**Step 2: Create the workflow file**

```yaml
name: Release

on:
  push:
    tags:
      - "v*"
  workflow_dispatch:
    inputs:
      tag:
        description: "Tag to release (e.g., v1.0.0). Leave empty to build snapshot."
        required: false
        type: string

permissions:
  contents: write

jobs:
  release:
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v4
        with:
          fetch-depth: 0

      - uses: actions/setup-go@v5
        with:
          go-version-file: go.mod

      - name: Set tag from manual input
        if: github.event_name == 'workflow_dispatch' && github.event.inputs.tag != ''
        run: |
          git tag ${{ github.event.inputs.tag }}

      - uses: goreleaser/goreleaser-action@v6
        with:
          version: "~> v2"
          args: >-
            release --clean
            ${{ github.event_name == 'workflow_dispatch' && github.event.inputs.tag == '' && '--snapshot' || '' }}
        env:
          GITHUB_TOKEN: ${{ secrets.GITHUB_TOKEN }}
```

**Step 3: Commit**

```bash
git add .github/workflows/release.yml
git commit -m "ci: add GitHub Actions release workflow"
```

---

### Task 3: Create install script

**Files:**
- Create: `install.sh`

**Step 1: Write the install script**

```bash
#!/bin/sh
set -eu

REPO="fireynis/ralph-loop-tui"
BINARY="ralph-loop-go"

# Detect OS
OS="$(uname -s)"
case "$OS" in
  Linux)  OS="linux" ;;
  Darwin) OS="darwin" ;;
  *)      echo "Unsupported OS: $OS" >&2; exit 1 ;;
esac

# Detect architecture
ARCH="$(uname -m)"
case "$ARCH" in
  x86_64)  ARCH="amd64" ;;
  aarch64) ARCH="arm64" ;;
  arm64)   ARCH="arm64" ;;
  *)       echo "Unsupported architecture: $ARCH" >&2; exit 1 ;;
esac

# Get latest release tag
TAG="$(curl -sSf "https://api.github.com/repos/${REPO}/releases/latest" | grep '"tag_name"' | cut -d '"' -f 4)"
VERSION="${TAG#v}"

echo "Installing ${BINARY} ${TAG} (${OS}/${ARCH})..."

# Build download URL
ARCHIVE="${BINARY}_${VERSION}_${OS}_${ARCH}.tar.gz"
URL="https://github.com/${REPO}/releases/download/${TAG}/${ARCHIVE}"
CHECKSUMS_URL="https://github.com/${REPO}/releases/download/${TAG}/checksums.txt"

# Create temp dir
TMPDIR="$(mktemp -d)"
trap 'rm -rf "$TMPDIR"' EXIT

# Download archive and checksums
curl -sSfL -o "${TMPDIR}/${ARCHIVE}" "$URL"
curl -sSfL -o "${TMPDIR}/checksums.txt" "$CHECKSUMS_URL"

# Verify checksum
cd "$TMPDIR"
if command -v sha256sum >/dev/null 2>&1; then
  grep "$ARCHIVE" checksums.txt | sha256sum -c --quiet
elif command -v shasum >/dev/null 2>&1; then
  grep "$ARCHIVE" checksums.txt | shasum -a 256 -c --quiet
else
  echo "Warning: no sha256sum or shasum found, skipping checksum verification" >&2
fi

# Extract
tar xzf "$ARCHIVE"

# Install
if [ -w /usr/local/bin ]; then
  INSTALL_DIR="/usr/local/bin"
elif [ -d "${HOME}/.local/bin" ]; then
  INSTALL_DIR="${HOME}/.local/bin"
else
  mkdir -p "${HOME}/.local/bin"
  INSTALL_DIR="${HOME}/.local/bin"
fi

mv "${BINARY}" "${INSTALL_DIR}/${BINARY}"
chmod +x "${INSTALL_DIR}/${BINARY}"

echo "Installed ${BINARY} to ${INSTALL_DIR}/${BINARY}"

# Check if install dir is in PATH
case ":$PATH:" in
  *":${INSTALL_DIR}:"*) ;;
  *) echo "Note: ${INSTALL_DIR} is not in your PATH. Add it with:" >&2
     echo "  export PATH=\"${INSTALL_DIR}:\$PATH\"" >&2 ;;
esac
```

**Step 2: Make it executable**

```bash
chmod +x install.sh
```

**Step 3: Commit**

```bash
git add install.sh
git commit -m "feat: add install script for easy binary installation"
```

---

### Task 4: Validate end-to-end

**Step 1: Review all three files are committed**

```bash
git log --oneline -5
```

Expected: Three new commits for goreleaser config, workflow, and install script.

**Step 2: Push to remote**

```bash
git push
```

**Step 3: Test with a tag (when ready)**

To cut a release:
```bash
git tag v0.1.0
git push origin v0.1.0
```

Then check GitHub Actions tab for the release workflow run. Once complete, verify:
- GitHub Release exists at `https://github.com/fireynis/ralph-loop-tui/releases/tag/v0.1.0`
- Four archives + checksums.txt are attached
- `curl -sSfL https://raw.githubusercontent.com/fireynis/ralph-loop-tui/main/install.sh | bash` works
