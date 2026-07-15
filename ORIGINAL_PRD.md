# Envapor PRD

## Overview

Envapor is a lightweight Git companion that transparently encrypts secrets stored in `.env` files. Plaintext secrets evaporate into Git — encrypted on commit, restored on checkout — with nothing extra to manage.

Developers continue editing `.env` exactly as they do today. Git automatically stores encrypted values before they are committed and automatically decrypts them during checkout.

There are no additional files (`.env.enc`), no wrapper commands, and no changes to how applications load configuration.

After a one-time setup, developers simply use Git as they always have.

## Problem

Managing `.env` files across teams is painful.

Current approaches include:

- Sharing `.env` files manually
- Maintaining `.env.example`
- Sending secrets through password managers or chat
- Manually copying secrets onto servers
- Introducing external secret-management infrastructure

This leads to:

- Missing variables
- Out-of-sync environments
- Difficult onboarding
- Manual deployments
- Human error

## Goals

- Zero workflow changes after setup
- Single `.env` file
- No `.env.enc`
- No `.env.example` — variable names stay readable in Git, so the committed file *is* the manifest
- Standard Git workflow
- Automatic encryption/decryption
- Human-readable Git diffs
- One-time setup
- Simple server provisioning

## Non-goals

Version 1 is **not** intended to replace:

- Hashicorp Vault
- AWS Secrets Manager
- Azure Key Vault
- Google Secret Manager

Envapor focuses on repository-based secret sharing.

## Design principles

- Single Go binary
- Cross-platform (macOS, Linux, Windows)
- Git-native
- Zero workflow changes
- Repository-local
- Secure by default
- No vendor lock-in

## User experience

### New repository

```bash
git init
envapor init --pem ~/keys/team.pem
```

`envapor init`:

- Configures Git clean/smudge filters
- Installs the pre-commit guard hook
- Creates `.gitattributes`
- Maps the repository to the supplied PEM
- Encrypts `.env`
- Verifies installation

After that:

```bash
git add
git commit
git push
git pull
```

Everything behaves like normal Git.

### Existing repository

```bash
git clone git@github.com:company/project.git
cd project
envapor init --pem ~/keys/team.pem
```

### Server setup

```bash
git clone git@github.com:company/project.git
cd project
envapor init --pem /etc/envapor/team.pem
docker compose up -d
```

No manual copying of `.env`.

## Managed files

Envapor manages `.env` and every `.env.*` variant by default, so framework conventions like `.env.local`, `.env.staging`, and `.env.production` work out of the box. Guarding only `.env` would silently leave those files — often the ones holding real production secrets — in plaintext, which is the worst possible default for a secrets tool.

Example/template files are explicitly **excluded** and never encrypted:

- `.env.example`
- `.env.sample`
- `.env.template`

These are conventionally committed as readable placeholders. Encrypting them would break that convention and clobber files other tooling and onboarding docs may still reference. Envapor leaves any pre-existing example file untouched.

Note that with Envapor, example files are largely obsolete: because variable names remain in plaintext in the committed `.env`, the committed file already serves as the manifest of required variables. The exclusion exists to avoid clobbering example files that predate Envapor, not to support an example-file workflow.

The exclusion list is user-extendable.

This is enforced through `.gitattributes`, written by `envapor init`:

```other
.env          filter=envapor diff=envapor
.env.*        filter=envapor diff=envapor
.env.example  -filter -diff
.env.sample   -filter -diff
.env.template -filter -diff
```

`envapor init` and `envapor doctor` report which files are currently managed, so coverage is never silent. `doctor` additionally verifies that example files are correctly excluded, since the negation order in `.gitattributes` is easy to get wrong.

## Core concept

Working tree:

```other
DATABASE_URL=postgres://...
STRIPE_KEY=sk_live_xxx
APP_ENV=production # PUBLIC
```

Repository:

```other
DATABASE_URL=ENC[...]
STRIPE_KEY=ENC[...]
APP_ENV=production # PUBLIC
```

Only values are encrypted.

Variable names remain plaintext.

## Public values

Any comment beginning with `PUBLIC` leaves the value unencrypted.

Examples:

```other
APP_ENV=production # PUBLIC
API_URL=https://api.example.com # PUBLIC: Browser endpoint
LOG_LEVEL=debug # PUBLIC - Safe to expose
```

Everything following `PUBLIC` is treated as documentation.

Parsing fails closed: if a line is ambiguous or the marker is malformed, the value is **encrypted**. A value is only left in plaintext on an unambiguous `PUBLIC` match.

## Encryption model

- Encrypt each value independently.
- Leave keys readable.
- Encryption is **deterministic**: the same value under the same key always produces the same ciphertext. This is what keeps diffs readable and merges clean — unchanged values produce unchanged ciphertext, so only edited variables change in Git.
- Encrypt everything unless explicitly marked `# PUBLIC`.

Stored format:

```other
DATABASE_URL=ENC[v1:...]
```

The `v1` prefix versions the format so the scheme can evolve without breaking existing repositories.

## Git integration

Envapor uses Git **clean/smudge filters** as the mechanism, backed by a **pre-commit hook** as a guard.

The filter does the work. The hook is a safety net that catches the case where the filter silently fails to run (for example, a filter not installed after a fresh clone) and aborts the commit before plaintext reaches the object store.

Working tree → commit:

```other
plaintext .env
      │
      ▼
Clean filter          (encrypts values)
      │
      ▼
Encrypted object stored in Git
```

Checkout:

```other
Encrypted object
      │
      ▼
Smudge filter         (decrypts values)
      │
      ▼
Plaintext .env
```

Pre-commit guard:

```other
Staged .env
      │
      ▼
Pre-commit hook       (verifies every non-PUBLIC value is ENC[...])
      │
      ├── ok ───────► commit proceeds
      │
      └── plaintext detected ─► abort commit
```

## Repository mapping

Each repository is mapped locally to a PEM.

Example:

```yaml
repos:
  git@github.com:automaze/api.git:
    key: automaze
```

Keys live under:

```other
~/.config/envapor/keys/
```

Mappings are local and never committed.

## Installation

### Homebrew

```bash
brew install automazeio/tap/envapor
```

### Go

```bash
go install github.com/automazeio/envapor@latest
```

### Binary

Download a release from GitHub.

## CLI

### Keygen

```bash
envapor keygen NAME
```

Generates a new key file with safe parameters by default, so users never have to assemble one from raw crypto tooling. This removes the friction of "bring your own PEM" and guarantees every generated key meets the tool's strength requirements rather than depending on the user's `openssl` knowledge.

The key is written to `~/.config/envapor/keys/NAME`.

### Initialize

```bash
envapor init --pem ~/keys/team.pem
```

### Doctor

```bash
envapor doctor
```

Checks:

- Git repository
- Filter installation
- Pre-commit hook installation
- Repository mapping
- Managed files (reports which `.env` files are covered)
- Example-file exclusions (verifies `.env.example` and friends are not encrypted)
- PEM availability
- Encryption
- Decryption

### Migrate

```bash
envapor migrate OLDPEM NEWPEM
```

Re-encrypts every managed value from the old key to a new one. Used when a team member leaves or a key is compromised.

Both keys appear in the command signature by design: migration inherently needs the old key to decrypt and the new key to re-encrypt, so the command states exactly what it does rather than relying on implicit current-key state.

**Scope and limits.** `migrate` re-encrypts the current working tree and all future commits. It does **not** rewrite Git history — values in past commits remain encrypted under the old key, and anyone who retains the old key and a prior clone can still decrypt that history. Migration is therefore a *key* rotation, not a *secret* rotation. After a compromise, the affected secrets themselves (database passwords, API keys) must also be rotated at their source. Envapor changes the lock; it cannot recall copies already distributed.

### Other commands

```other
envapor status
envapor encrypt
envapor decrypt
```

## Merge behavior

Because each value is encrypted independently and deterministically:

- Only changed variables change in Git.
- Concurrent edits to different variables merge naturally.
- Variable names remain visible.

## Security assumptions

- Team members are trusted.
- PEMs are distributed securely.
- PEMs are never committed.
- Offboarding is an organizational responsibility: when someone leaves, the org rotates the affected secrets and re-issues the PEM. Envapor does not manage this.

## CI/CD

Envapor ships with a first-party GitHub Action.

Example:

```yaml
- uses: automazeio/setup-envapor@v1
- run: envapor init --pem "${{ secrets.ENVAPOR_KEY }}"
- run: go test ./...
```

The action installs Envapor, imports the key, configures Git filters, installs the guard hook, and decrypts the repository.

## Future ideas

- External secret providers (1Password, Vault, AWS Secrets Manager)
- Multiple environment files
- CI verification (`envapor verify`)

## Success criteria

A new developer should be able to run:

```bash
git clone ...
cd repo
envapor init --pem ~/keys/team.pem
```

…and never think about Envapor again.
