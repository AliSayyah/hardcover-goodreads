# Hardcover to Goodreads backup

[![CI](https://github.com/AliSayyah/hardcover-goodreads/actions/workflows/ci.yml/badge.svg)](https://github.com/AliSayyah/hardcover-goodreads/actions/workflows/ci.yml)

Goodreads is the weak side of this sync: the public API is deprecated and new API
keys are not issued. This repo keeps Hardcover as the source of truth and creates
a Goodreads import CSV you upload manually.

## Install

macOS/Linux:

```bash
curl -fsSL https://raw.githubusercontent.com/AliSayyah/hardcover-goodreads/main/install.sh | sh
```

Make sure `~/.local/bin` is on your `PATH`.

Windows PowerShell:

```powershell
irm https://raw.githubusercontent.com/AliSayyah/hardcover-goodreads/main/install.ps1 | iex
```

The Windows installer adds the app to your user `PATH`.

Then run:

```bash
hardcover-goodreads
```

It opens <http://localhost:8080> automatically.

## Quick Start

1. Run `hardcover-goodreads`.
2. Click "Open Hardcover API Key Page" in the app, or open <https://hardcover.app/account/api>.
3. Paste the Hardcover token into the local page.
4. Check "Save token in OS keychain" if you want to reuse it later.
5. Export the CSV, then upload it at <https://www.goodreads.com/review/import>.

The JSON download is the full local backup.

## Usage

```bash
hardcover-goodreads                 # start on :8080, or the next free port, and open the browser
hardcover-goodreads -no-open        # start without opening the browser
hardcover-goodreads -addr :9090     # use a different port
hardcover-goodreads -version
```

## Development

Run from source:

```bash
go run -buildvcs=false .
```

Run the checks:

```bash
go test ./...
go build -buildvcs=false ./...
```

## Why this shape

- One-way only: Hardcover -> Goodreads.
- No Goodreads password stored locally.
- Hardcover token storage uses the OS keychain: macOS Keychain, Windows Credential Manager, or Linux Secret Service.
- No browser automation that breaks when Goodreads changes HTML or asks for MFA.

## Release

Push a tag and GitHub Actions will publish binaries:

```bash
git tag vX.Y.Z
git push origin vX.Y.Z
```
