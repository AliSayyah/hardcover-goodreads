# Hardcover to Goodreads backup

[![CI](https://github.com/AliSayyah/hardcover-goodreads/actions/workflows/ci.yml/badge.svg)](https://github.com/AliSayyah/hardcover-goodreads/actions/workflows/ci.yml)

Goodreads is the weak side of this sync: the public API is deprecated and new API
keys are not issued. This repo keeps Hardcover as the source of truth and creates
a Goodreads import CSV you upload manually.

## Install

```bash
curl -fsSL https://raw.githubusercontent.com/AliSayyah/hardcover-goodreads/main/install.sh | sh
```

Then run:

```bash
hardcover-goodreads
```

## Use

1. Get your Hardcover token from <https://hardcover.app/account/api>.
2. Run from source:

```bash
go run -buildvcs=false .
```

3. Open <http://localhost:8080>.
4. Export the CSV, then upload it at <https://www.goodreads.com/review/import>.

The JSON download is the full local backup.
Check "Save token in OS keychain" after pasting your token once to reuse it later.

## Why this shape

- One-way only: Hardcover -> Goodreads.
- No Goodreads password stored locally.
- Hardcover token storage uses the OS keychain: macOS Keychain, Windows Credential Manager, or Linux Secret Service.
- No browser automation that breaks when Goodreads changes HTML or asks for MFA.

Run the check with:

```bash
go test ./...
go build -buildvcs=false ./...
```

## Release

Push a tag and GitHub Actions will publish binaries:

```bash
git tag v0.1.0
git push origin v0.1.0
```
