<h1 align="center">envapor</h1>

<p align="center">Commit your secrets, securely.</p>

<p align="center">
  <a href="https://github.com/automazeio/envapor/actions/workflows/ci.yml"><img src="https://github.com/automazeio/envapor/actions/workflows/ci.yml/badge.svg" alt="CI"></a>
  <a href="https://github.com/automazeio/envapor/releases"><img src="https://img.shields.io/github/v/release/automazeio/envapor?sort=semver" alt="Release"></a>
  <a href="./LICENSE"><img src="https://img.shields.io/badge/license-Apache--2.0-blue" alt="License"></a>
</p>

---

Envapor transparently encrypts the **values** in your `.env` files inside Git. Plaintext is encrypted on commit and restored on checkout, so your working tree stays exactly as it is today while Git only ever stores ciphertext.

No `.env.enc`, no wrapper commands, no changes to how your app loads config. After a one-time setup you just use Git.

```diff
  # your working tree (.env)                 # what Git actually stores
  DATABASE_URL=postgres://user:pass@db/app   DATABASE_URL=ENC[v2:9f3a…]
  STRIPE_KEY=sk_live_51H8xY2…                STRIPE_KEY=ENC[v2:c17b…]
  APP_ENV=production # PUBLIC                APP_ENV=production # PUBLIC
```

Variable **names stay readable**, so the committed `.env` is its own manifest. No `.env.example` to maintain.

## Why

- **Zero workflow change.** `git add/commit/push/pull` work unchanged after setup.
- **One file.** A single `.env`, no parallel encrypted copy to keep in sync.
- **Readable diffs & clean merges.** Encryption is deterministic, so only variables you actually change show up in a diff, and edits to different keys merge without conflict.
- **Git-native.** Built on Git clean/smudge filters plus a pre-commit guard. Nothing to run at runtime.
- **Single static binary.** macOS, Linux, Windows. No dependencies.

## Install

```bash
# macOS (Homebrew)
brew install automazeio/tap/envapor

# Linux
curl -fsSL https://raw.githubusercontent.com/automazeio/envapor/main/installers/install.sh | sh

# Windows (PowerShell)
irm https://raw.githubusercontent.com/automazeio/envapor/main/installers/install.ps1 | iex

# From source (Go)
git clone https://github.com/automazeio/envapor
cd envapor/src && go install .
```

Verify with `envapor --version`.

## Quick start

```bash
# once per team: generate a shared key
envapor keygen team

# in a repo (new or freshly cloned)
envapor init team              # uses ~/.config/envapor/keys/team
# or import a key file from anywhere:
envapor init --pem /path/to/team.pem

# then just use git
git add .env
git commit -m "Add config"
git push
```

On clone, teammates run the same `envapor init team` (or `envapor init --pem …`) and Git decrypts `.env` in place. Same command provisions servers and CI, no manual copying of secrets.

## Usage

Edit `.env` normally. Values are encrypted on commit and decrypted on checkout automatically.

**Public values** — anything you want left readable in Git is marked with a `PUBLIC` comment:

```bash
APP_ENV=production            # PUBLIC
API_URL=https://api.acme.com  # PUBLIC: browser endpoint
```

Parsing **fails closed**: a value is left in plaintext *only* on an unambiguous `# PUBLIC` marker. Anything ambiguous is encrypted.

**Managed files** — `.env` and every `.env.*` variant are managed by default (`.env.local`, `.env.production`, …). Template files (`.env.example`, `.env.sample`, `.env.template`) are always excluded. Coverage is written to `.gitattributes` by `envapor init`.

### Commands

| Command | Purpose |
|---|---|
| `envapor keygen NAME` | Generate a new key at `~/.config/envapor/keys/NAME` |
| `envapor keys` | List stored keys, marking the current repository's key |
| `envapor init NAME` or `--pem PATH` | Configure filters, hook, `.gitattributes`, and map the repo to a key (by stored name or key file) |
| `envapor doctor` | Diagnose the setup (filters, hook, mapping, coverage, crypto round-trip) |
| `envapor status` | Show the mapping and per-file encryption state |
| `envapor migrate OLDPEM NEWPEM` | Re-encrypt managed values from one key to another |
| `envapor encrypt` / `decrypt` | Manually transform managed files (rarely needed) |

## How it works

Git **clean/smudge filters** do the work: the clean filter encrypts values on the way into the object store; the smudge filter decrypts them on checkout. A **pre-commit hook** is a safety net that aborts the commit if any non-`PUBLIC` value would reach Git as plaintext (for example, before filters are installed on a fresh clone).

Each value is encrypted independently with **AES-256-GCM**, using encryption/MAC subkeys derived from your 512-bit master key via **HKDF**. Encryption is deterministic (SIV-style: the nonce is derived from the variable name and the plaintext), which keeps diffs readable and merges clean. Each token is also **bound to its variable name**, so a ciphertext moved to a different variable fails to decrypt rather than silently supplying the wrong secret. Tokens are versioned (`ENC[v2:…]`) so the format can evolve; older `v1` tokens remain readable.

## Security

Read this before adopting Envapor. It makes explicit trade-offs.

- **Trust model.** The master key is symmetric and shared. Every key holder can decrypt everything. Envapor is for **trusted teams sharing a repo**, not fine-grained or per-secret access control.
- **Deterministic encryption leaks equality.** The same value under the same key and variable name always yields the same ciphertext. Anyone with read access to the encrypted repo (no key needed) can tell whether a value is unchanged across commits or whether the same variable holds the same value in two files. This is the deliberate cost of clean diffs and merges. If value-equality leakage is unacceptable for you, Envapor is the wrong tool.
- **Keys never touch the repo.** They live under `~/.config/envapor/keys/` and repo→key mappings are stored locally. Distribute keys over a secure channel (e.g., a password manager), and never commit them.
- **Key rotation ≠ secret rotation.** `envapor migrate` re-encrypts the working tree and future commits under a new key. It does **not** rewrite history: past commits stay encrypted under the old key, so anyone holding the old key and an old clone can still read that history. After a compromise, you must also **rotate the affected secrets at their source** (DB passwords, API keys). Envapor changes the lock; it can't recall copies that have already been distributed.
- **Offboarding is on you.** When someone leaves, rotate the affected secrets and re-issue the key. Envapor doesn't manage this.

Envapor is **not** a replacement for HashiCorp Vault, AWS Secrets Manager, Azure Key Vault, or Google Secret Manager. It targets repository-based secret sharing, and complements those systems rather than competing with them.

## CI/CD

A first-party GitHub Action installs Envapor, imports the key, configures filters, and decrypts the repo:

```yaml
- uses: automazeio/setup-envapor@v1
  with:
    key: ${{ secrets.ENVAPOR_KEY }}
- run: go test ./...
```

Store the PEM contents in your CI provider's encrypted secrets.

## Documentation

Full walkthrough, team/server/CI workflows, and troubleshooting live in [`docs/user-guide.md`](./docs/user-guide.md).

## License

[Apache-2.0](./LICENSE)
