---
name: envapor
description: Manage encrypted .env files in Git using Envapor (transparent AES-256-GCM encryption via clean/smudge filters). Use when a repository contains ENC[...] values in .env files, when asked to encrypt .env secrets in Git, onboard to or set up an Envapor repository, rotate Envapor keys, or debug Envapor filter, hook, or decryption issues.
---

# Envapor

Envapor keeps `.env` files plaintext in the working tree while Git stores only
encrypted values (`KEY=ENC[v2:...]`). Variable names stay readable; only values
are encrypted. Filters run automatically on commit/checkout, and a pre-commit
guard blocks any plaintext secret from reaching Git.

## Detect

- A repo uses Envapor when `.gitattributes` contains `filter=envapor`, or
  committed `.env` values look like `ENC[v2:...]` (or legacy `ENC[v1:...]`).
- Check the binary: `command -v envapor`; verify setup: `envapor doctor`.

## Install (when missing)

```bash
# macOS
brew install automazeio/tap/envapor
# Linux
curl -fsSL https://raw.githubusercontent.com/automazeio/envapor/main/installers/install.sh | sh
# Windows (PowerShell)
irm https://raw.githubusercontent.com/automazeio/envapor/main/installers/install.ps1 | iex
# From source
git clone https://github.com/automazeio/envapor && cd envapor/src && go install .
```

## Set up a repository

1. **Key.** For a brand-new setup: `envapor keygen NAME` (once per team).
   For an existing Envapor repo: **ask the user for the team key** — a freshly
   generated key cannot decrypt existing data.
2. **Init.** `envapor init NAME` (uses `~/.config/envapor/keys/NAME`) or
   `envapor init --pem PATH` to import a key file. This configures filters,
   installs the guard hook, writes `.gitattributes`, and decrypts in place.
3. **Verify.** `envapor doctor` must pass every check.

`.env` and every `.env.*` variant are managed automatically;
`.env.example|sample|template` are excluded.

## Everyday use

Plain `git add/commit/push/pull` — encryption is automatic. Useful commands:

- `envapor keys` — list stored keys; marks the current repo's key
- `envapor status` — per-file encryption state (flags `PLAINTEXT in index`)
- `envapor migrate OLDKEY NEWKEY` — rotate to a new key (stored names or PEM
  paths). History stays under the old key; after a compromise the underlying
  secrets must also be rotated at their source.
- `envapor encrypt` / `envapor decrypt` — manual transforms (rarely needed)

Mark non-secret values to keep them readable in Git: `APP_ENV=production # PUBLIC`

## CI

```yaml
- uses: automazeio/setup-envapor@v1
  with:
    key: ${{ secrets.ENVAPOR_KEY }}   # PEM contents from CI secrets
```

## Troubleshooting

- **`ENC[v2:...]` values in the working tree**, or `git pull` printed
  `envapor: warning: could not decrypt ...`: the git operation succeeded but
  the local key is missing or wrong. Fix: `envapor init <correct-key>`, then
  `envapor decrypt`.
- **Commit blocked by the pre-commit guard**: the clean filter didn't run.
  Run `envapor doctor` and fix what it reports; never bypass with `--no-verify`.
- **`is neither a stored key nor a PEM file`**: run `envapor keys` to see
  stored names, or pass a path (prefix `./` to force file interpretation).

## Safety rules

- **Never** commit, print, or log key files (`~/.config/envapor/keys/*`) or
  PEM contents — treat them as secrets, distributed only via secure channels.
- **Never** add `.env` to `.gitignore` in an Envapor repo (it must be tracked
  to be encrypted and shared).
- **Never** bypass the pre-commit hook to force a commit through.
- Working-tree `.env` files are plaintext by design — do not paste their
  contents into logs, PRs, or chat.
