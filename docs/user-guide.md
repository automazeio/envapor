# Envapor User Guide

Envapor is a lightweight Git companion that transparently encrypts the secrets in your `.env` files. You keep editing `.env` exactly as you do today; Git stores encrypted values on commit and restores plaintext on checkout. There are no extra files to manage, no wrapper commands, and no changes to how your application loads configuration.

After a one-time setup, you just use Git the way you always have.

---

## Table of contents

1. [How it works](#how-it-works)
2. [Installation](#installation)
3. [Quick start](#quick-start)
4. [Generating a key](#generating-a-key)
5. [Everyday workflow](#everyday-workflow)
6. [Public values](#public-values)
7. [Which files are managed](#which-files-are-managed)
8. [Command reference](#command-reference)
9. [Team workflows](#team-workflows)
10. [Server and deployment setup](#server-and-deployment-setup)
11. [CI/CD](#cicd)
12. [Rotating keys](#rotating-keys)
13. [Troubleshooting](#troubleshooting)
14. [FAQ](#faq)

---

## How it works

Envapor hooks into Git using **clean/smudge filters**, backed by a **pre-commit guard hook**.

- The **clean filter** encrypts each value as a file is staged, so only ciphertext ever reaches Git's object store.
- The **smudge filter** decrypts values on checkout, so your working copy of `.env` is always plaintext.
- The **pre-commit hook** is a safety net: if the filter ever fails to run (for example, right after a fresh clone before setup), the hook aborts the commit before any plaintext secret is committed.

Only **values** are encrypted. **Variable names stay readable** in Git, so the committed `.env` doubles as the manifest of required variables. No `.env.example` needed.

Your working tree:

```
DATABASE_URL=postgres://user:pass@localhost/app
STRIPE_KEY=sk_live_51H8xY2...
APP_ENV=production # PUBLIC
```

What Git stores:

```
DATABASE_URL=ENC[v2:9f3a...]
STRIPE_KEY=ENC[v2:c17b...]
APP_ENV=production # PUBLIC
```

Encryption is **deterministic**: the same value under the same key and variable name always produces the same ciphertext. That keeps diffs readable and merges clean; only the variables you actually change show up as changes in Git.

Each token is also **bound to its variable name**. Copying an `ENC[...]` value from one variable to another (or renaming a variable directly in the encrypted file) makes decryption fail with an authentication error instead of silently supplying the wrong secret. Rename variables in your plaintext working tree as usual — the filter re-encrypts them under the new name on commit.

---

Envapor is a single, self-contained binary with no runtime dependencies, available for macOS, Linux, and Windows. Pick the method for your platform.

### macOS — Homebrew

```bash
brew install automazeio/tap/envapor
```

### Linux — install script

```bash
curl -fsSL https://raw.githubusercontent.com/automazeio/envapor/main/installers/install.sh | sh
```

The script detects your CPU architecture (`amd64` or `arm64`), downloads the matching release from GitHub, verifies its checksum against the published `checksums.txt`, and installs the `envapor` binary to a directory on your `PATH`.

Defaults and overrides (environment variables):

- `ENVAPOR_VERSION` — install a specific release tag (for example `v1.2.3`) instead of the latest.
- `ENVAPOR_INSTALL_DIR` — target directory. Defaults to `/usr/local/bin` when writable, otherwise `~/.local/bin`.

To review the script before running it, open the URL in a browser or `curl` it to a file first.

### Windows — PowerShell

```powershell
irm https://raw.githubusercontent.com/automazeio/envapor/main/installers/install.ps1 | iex
```

The PowerShell installer detects your architecture, downloads and verifies the matching release, installs `envapor.exe` under `%LOCALAPPDATA%\Envapor\bin`, and adds that directory to your user `PATH`. Open a new terminal afterward so the updated `PATH` takes effect.

To pin a version, set `$env:ENVAPOR_VERSION` before running the command.

### Any platform — download a release binary

Download the archive for your OS and architecture from the [GitHub releases page](https://github.com/automazeio/envapor/releases), extract it, and place the `envapor` binary somewhere on your `PATH`.

### For Go developers

If you already have a Go toolchain, you can build and install from source. This lives alongside the platform installers above and is not required for normal use:

```bash
git clone https://github.com/automazeio/envapor
cd envapor/src && go install .
```

### Verify the install

```bash
envapor --version
```

---

## Quick start

### Brand-new repository

```bash
git init
envapor keygen team          # create a key (once per team)
envapor init team            # uses ~/.config/envapor/keys/team
```

Then use Git normally:

```bash
git add .env
git commit -m "Add environment configuration"
git push
```

### Existing repository

```bash
git clone git@github.com:company/project.git
cd project
envapor init team            # or: envapor init --pem /path/to/team.pem
```

That's it. From here, `git add`, `git commit`, `git push`, and `git pull` all behave exactly like standard Git.

---

## Generating a key

Envapor generates keys for you with safe parameters, so you never assemble one from raw crypto tooling:

```bash
envapor keygen NAME
```

This writes the key to:

```
~/.config/envapor/keys/NAME
```

Every generated key is a **512-bit random master key** drawn from the operating system's CSPRNG; the AES-256-GCM encryption and HMAC-SHA256 subkeys are derived from it via HKDF, so no primitive ever uses the master directly. Keys from earlier Envapor versions (256-bit) remain fully supported. Distribute the key to teammates through a secure channel (see [Team workflows](#team-workflows)). **Never commit a key to the repository.**

---

## Everyday workflow

Once a repository is initialized, there is nothing new to learn. Edit `.env` in your editor as usual:

```bash
# open .env, change a value, save
git add .env
git commit -m "Rotate Stripe key"
git push
```

- On **commit**, values are encrypted automatically.
- On **checkout / pull**, values are decrypted automatically.
- Your app reads `.env` exactly as before; Envapor is invisible at runtime.

To see the current state at any time:

```bash
envapor status
```

---

## Public values

Some values are not secret and are more useful left readable in Git (environment names, public API URLs, log levels). Mark these with a `PUBLIC` comment:

```
APP_ENV=production # PUBLIC
API_URL=https://api.example.com # PUBLIC: Browser endpoint
LOG_LEVEL=debug # PUBLIC - Safe to expose
```

Everything after `PUBLIC` is treated as free-form documentation.

**Envapor fails closed.** If a line is ambiguous or the marker is malformed, the value is **encrypted**. A value is left in plaintext only on an unambiguous `PUBLIC` match. When in doubt, Envapor protects the secret.

---

## Which files are managed

By default, Envapor manages `.env` and every `.env.*` variant, so framework conventions like `.env.local`, `.env.staging`, and `.env.production` work out of the box.

**Example / template files are explicitly excluded and never encrypted:**

- `.env.example`
- `.env.sample`
- `.env.template`

These are conventionally committed as readable placeholders, so Envapor leaves any pre-existing example file untouched. (With Envapor, example files are largely obsolete: since variable names stay in plaintext, the committed `.env` already lists every required variable.)

Coverage is enforced through `.gitattributes`, written by `envapor init`:

```
.env          filter=envapor diff=envapor
.env.*        filter=envapor diff=envapor
.env.example  -filter -diff
.env.sample   -filter -diff
.env.template -filter -diff
```

Both `envapor init` and `envapor doctor` report which files are currently managed, so coverage is never silent, and `doctor` verifies the example-file exclusions are correct.

`envapor init` owns the block between its `# >>> envapor >>>` / `# <<< envapor <<<` markers and rewrites it on every run. To add your own rules (for example, excluding an extra template file), place them **outside** those markers in `.gitattributes`; anything inside the managed block is regenerated and will be overwritten.

If a managed `.env` file is git-ignored, it would silently never be committed. Both `envapor init` and `envapor doctor` detect this and warn you to remove the entry from `.gitignore`.

---

## Command reference

### `envapor keygen NAME`

Generates a new key with safe defaults and writes it to `~/.config/envapor/keys/NAME`. Refuses to overwrite an existing key unless you pass `--force`.

### `envapor keys`

Lists the keys stored in the keys directory (`~/.config/envapor/keys/` by
default), one per line. When run inside an initialized repository, the key
mapped to that repository is marked. Prints a hint instead when no keys exist
yet.

### `envapor init [KEY_NAME]` / `envapor init --pem PATH`

Sets up Envapor in the current repository. The key is given either as the name
of a key already stored in the keys directory (`envapor init team` uses
`~/.config/envapor/keys/team`, and fails if no such key exists) or as `--pem`
with the path to a PEM key file anywhere on disk, which is imported into the
keys directory. Passing both is an error. It:

- Configures the Git clean/smudge filters
- Installs the pre-commit guard hook
- Creates `.gitattributes`
- Maps the repository to the supplied key
- Encrypts existing `.env` files
- Verifies the installation

If a different (non-Envapor) pre-commit hook already exists, `init` stops rather than clobber it; re-run with `--force` to replace it.

### `envapor doctor`

Runs a full health check and reports on:

- Git repository detection
- Filter installation
- Pre-commit hook installation
- Repository mapping
- Managed files (which `.env` files are covered)
- Example-file exclusions (`.env.example` and friends are not encrypted)
- Key availability
- Encryption round-trip
- Decryption round-trip

Run this first whenever something looks off.

### `envapor migrate OLDPEM NEWPEM`

Re-encrypts every managed value from the old key to a new one. Both arguments are paths to PEM key files. Used when a teammate leaves or a key is compromised. See [Rotating keys](#rotating-keys) for scope and limits.

### `envapor status`

Shows the current Envapor state for the repository.

### `envapor encrypt`

Manually encrypts managed files (rarely needed; the filter does this automatically).

### `envapor decrypt`

Manually decrypts managed files (rarely needed; the filter does this automatically).

---

## Team workflows

Envapor is designed around **repository-based secret sharing** among trusted teammates.

1. One person runs `envapor keygen team` to create the team key.
2. That key is distributed securely (password manager, encrypted channel, or your org's secret distribution process). **The key is never committed.**
3. Each teammate saves the key under `~/.config/envapor/keys/` and runs `envapor init team` after cloning (or `envapor init --pem <path>` if the key file lives elsewhere).

Repository-to-key mappings are stored locally and never committed:

```yaml
repos:
  git@github.com:automaze/api.git:
    key: team
```

Because encryption is deterministic and per-value:

- Only changed variables change in Git.
- Concurrent edits to *different* variables merge naturally.
- Variable names stay visible in diffs and history.

---

## Server and deployment setup

No manual copying of `.env` onto servers. Provision exactly like a developer machine:

```bash
git clone git@github.com:company/project.git
cd project
envapor init --pem /etc/envapor/team.pem
docker compose up -d
```

The checkout decrypts `.env` in place, and your application loads it as usual.

---

## CI/CD

Envapor ships with a first-party GitHub Action that installs the tool, imports the key, configures the Git filters, installs the guard hook, and decrypts the repository. Pass the key **contents** through the action's required `key` input; the action runs `envapor init` for you, so no separate init step is needed:

```yaml
- uses: automazeio/setup-envapor@v1
  with:
    key: ${{ secrets.ENVAPOR_KEY }}
- run: go test ./...
```

Store the PEM key contents in your CI provider's encrypted secrets (for example, `ENVAPOR_KEY` above).

The action accepts two optional inputs:

- `version` — the Envapor release to install (a tag like `v1.2.3`, or `latest`; defaults to `latest`).
- `key-name` — the name the imported key is stored under (defaults to `ci`).

---

## Rotating keys

When a teammate leaves or a key is compromised:

```bash
envapor migrate OLDPEM NEWPEM
```

Both arguments are paths to PEM key files. This re-encrypts the current working tree and all future commits under the new key.

**Important scope and limits:**

- `migrate` performs a **key** rotation, not a **secret** rotation.
- It does **not** rewrite Git history. Values in past commits remain encrypted under the old key, and anyone who kept the old key and an old clone can still decrypt that history.
- After a compromise, you must also **rotate the affected secrets at their source** (database passwords, API keys, etc.). Envapor changes the lock; it cannot recall copies already distributed.

Both keys appear in the command by design: migration needs the old key to decrypt and the new key to re-encrypt, so the command states exactly what it does.

---

## Troubleshooting

**Values look like `ENC[v2:...]` in my editor.**
The smudge filter did not run on checkout. Run `envapor init --pem <key>` to reinstall the filters, then re-checkout the file with `git checkout -- .env`.

**My commit was aborted with a plaintext warning.**
This is the pre-commit guard doing its job: it caught a value that was not encrypted. Run `envapor doctor` to confirm the filters are installed, then re-stage and commit.

**A value I marked `PUBLIC` is still encrypted.**
Envapor fails closed on ambiguous markers. Check that the comment reads exactly `# PUBLIC` (optionally followed by `:` or `-` and a description). Any malformed marker causes the value to be encrypted.

**Envapor warns that my `.env` is git-ignored.**
A git-ignored `.env` would never be committed, so its secrets would silently stay only on your machine. Remove the entry from `.gitignore` so the file is tracked; Envapor still encrypts it on commit, so ignoring it is unnecessary and unsafe.

**Setup looks wrong / something is off.**
Run:

```bash
envapor doctor
```

It reports on every part of the installation (filters, hook, mapping, managed files, exclusions, key, and encryption/decryption round-trips).

---

## FAQ

**Do I need a `.env.example` file?**
No. Variable names stay readable in the committed `.env`, so the file itself is the manifest. Existing example files are left untouched.

**Does Envapor change how my app reads configuration?**
No. Your working-tree `.env` is always plaintext, so your app loads it exactly as before.

**Is Envapor a replacement for Vault / AWS Secrets Manager / Azure Key Vault?**
No. Envapor focuses on repository-based secret sharing for trusted teams. It is complementary to those systems, not a replacement.

**Where are keys stored?**
Under `~/.config/envapor/keys/` by default. The location follows `ENVAPOR_HOME` if set, then `XDG_CONFIG_HOME/envapor`, and on Windows falls back to `%APPDATA%\envapor`. Keys and repository mappings are local and never committed.

**What does the `v2` prefix in `ENC[v2:...]` mean?**
It versions the encryption format so the scheme can evolve without breaking existing repositories. Current tokens are `v2`, which binds each value to its variable name. Repositories encrypted by earlier versions may still contain `v1` tokens; they remain readable and upgrade to `v2` naturally as values change.

**Success looks like this:** a new developer runs `git clone`, `cd repo`, `envapor init --pem <key>`, and never thinks about Envapor again.
