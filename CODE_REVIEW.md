# Code Review: envapor

**Date:** 2026-07-15
**Scope:** Full codebase — `src/` (~1,400 lines of Go), `.github/` workflows and setup action, `.goreleaser.yaml`
**Method:** Three parallel specialized reviews (security, correctness/quality, performance), with every critical and high finding independently verified against the source.

**TL;DR:** The core design is sound, but two verified parser bugs let secrets reach git as plaintext while the pre-commit safety net stays silent. Those must be fixed before this tool can be trusted for its stated purpose.

---

## Critical — plaintext secrets bypass both encryption and the guard

### 1. A UTF-8 BOM (or any non-ASCII leading byte) disables encryption for that line

`src/internal/envfile/parse.go:77` (`scanKey`)

`scanKey` accepts only ASCII, so `\xEF\xBB\xBFSECRET=hunter2` is classified as a non-assignment line: `Encrypt` passes it through untouched, and `Verify` (the check backing the pre-commit guard) reports zero violations. The secret is committed in plaintext with no warning. Editors on Windows commonly add BOMs.

**Fix:** Strip/tolerate a BOM in `splitLines`, and consider having `Verify` flag any line it cannot parse in a managed file.

### 2. Multiline quoted values leak their continuation lines

`src/internal/envfile/parse.go:99` (`splitValue`)

The parser is strictly per-physical-line. For `MULTI="line1␤line2 secret"`, the first line's unterminated quote is encrypted (the "fail closed" path works there), but `line2 secret"` parses as a non-assignment line — committed verbatim in plaintext, and again invisible to `Verify`. Multiline values are a standard dotenv feature; private keys and certificates are the classic case, i.e. exactly the highest-value secrets.

**Root cause shared with #1:** unparseable lines are treated as "not my problem" instead of "potential secret." For a secrets tool, `Verify` should fail closed on anything in a managed file it cannot positively classify as safe.

---

## High

### 3. Path traversal via key name

`src/internal/cmd/keygen.go:18` + `src/internal/config/config.go` (`KeyPath`, `WriteKey`, `LoadKey`, `KeyExists`)

`keygen` passes the raw CLI argument into `filepath.Join(KeysDir(), name)` with no validation, so `envapor keygen ../../../path/x` writes key material outside the keys directory. `LoadKey` has the same issue via the `envapor.key` git config value, which is attacker-influenced if a `.git/config` ships inside an archive or image. `init`/`migrate` already sanitize via `filepath.Base` — `keygen` and the config layer should too. (Flagged independently by both the security and correctness reviewers.)

### 4. The pre-commit guard fails open

`src/internal/cmd/hook.go:37`

Any error from `gitutil.ShowStaged(f)` hits `continue`, silently skipping the plaintext check for that file. A transient `git show` failure defeats the guard's whole purpose. It should collect the error and block the commit.

### 5. `IsEncrypted` is purely syntactic

`src/internal/crypto/cipher.go:90`

Anything shaped like `ENC[v1:...]` is skipped by `Encrypt` and accepted by `Verify` without checking that the payload is even valid base64. A plaintext value that happens to match the shape gets committed as-is (and later breaks the smudge filter). At minimum, validate that the payload decodes and has a plausible length.

---

## Medium

### 6. No AAD context binding

`src/internal/crypto/cipher.go:35`

Encryption is deliberately deterministic (documented and reasonable for mergeable diffs), but `Seal` passes `aad=nil`, with two consequences:

- **Equality leakage:** identical values anywhere in the repo produce identical tokens, so anyone with read access to the encrypted repo can detect secret reuse across environments/services without the key.
- **Token substitution:** someone with repo *write* access but no key can swap `ENC[...]` tokens between variables (e.g. staging ↔ prod `DB_PASSWORD`) and decryption succeeds silently — an integrity attack that AEAD normally prevents via AAD.

Binding the variable name as AAD fixes substitution and most of the equality leakage, at the cost of tokens changing when a variable is renamed — worth the trade for a security tool, or at least a documented decision.

### 7. Fresh-clone flow doesn't match the docs

`src/internal/cmd/init.go` + `.github/actions/setup-envapor/action.yml` + `docs/user-guide.md`

Filter config is local-only (never committed), so on a fresh clone files land as raw `ENC[...]` and `envapor init` never re-smudges the working tree — but the docs and the setup action claim checkout/init decrypts. It fails safe (no leak), but CI jobs and deploys following the documented flow load literal `ENC[...]` strings as env values. `init` should force a re-checkout of managed files after configuring filters.

### 8. `migrate` is not atomic

`src/internal/cmd/migrate.go:45`

A mid-loop failure leaves some files normalized and the key mapping unswitched, with no rollback. Not a plaintext *leak* — the working tree is plaintext by design under smudge — but the partial state is confusing to recover from.

### 9. Zero tests for `internal/cmd`, `internal/config`, `internal/gitutil`

Only `crypto` and `envfile` have test files, and findings #3, #4, and #8 all live in the untested packages.

### 10. Permissions enforced only at creation

`src/internal/config/config.go:94`

`os.WriteFile(..., 0o600)` does not chmod a pre-existing file; a key file restored from a backup with loose permissions stays group/world-readable, and `doctor` doesn't check for it.

---

## Low / notes

- **Performance (acceptable for v1, worth knowing):** filters use per-file exec rather than git's long-running `process` protocol, and each invocation shells out to `git config` for the key name — so every git operation costs two process spawns per managed file (`src/internal/gitutil/filters.go:16`, `src/internal/cmd/common.go:20`). `Encrypt`/`Decrypt` also rebuild the AES-GCM AEAD per value (`src/internal/crypto/cipher.go:70`); cache it on `Key`.
- `.github/actions/setup-envapor/action.yml:50` — the downloaded binary is never verified against the published `checksums.txt`; the CI key file is briefly created with default umask before `chmod 600`.
- CI actions are tag-pinned (`@v4`) rather than SHA-pinned — a standard supply-chain hardening gap.
- `shellQuote` (`src/internal/gitutil/filters.go:5`, `hook.go:25`) escapes only `"` — `$`, backticks, and `\` stay live inside double quotes. Only exploitable via a hostile install path, but easy to harden.
- `--force` hook install (`src/internal/gitutil/hook.go:38`) discards a user's existing pre-commit hook with no backup.
- Config `Save` (`src/internal/config/config.go:95`) isn't atomic (no temp-file + rename), so concurrent invocations sharing a config dir can lose updates.
- `doctor` and `status` issue one git subprocess per file/attribute where a batched call would do (`src/internal/cmd/doctor.go:90`, `src/internal/cmd/status.go:39`).
- Key material (`Key.master/enc/mac`) is never zeroized in memory — standard caveat for Go crypto CLIs, low impact for a short-lived process.

---

## What's good

The crypto core is genuinely well done: AES-256-GCM with HKDF subkey separation, an HMAC-derived synthetic nonce (SIV-style) so nonce reuse only occurs for identical plaintext, a versioned token format, and clear comments explaining the determinism trade-off. All `git` calls use argument slices with `--` separators (no shell/argument injection), workflow permissions are minimal, the envfile round-tripping preserves CRLF and missing-final-newline byte-exactly, and `go vet`, `go build`, and `go test -race` all pass.

---

## Verdict

❌ **Changes requested.** Fix the two parser leak paths (#1, #2) and make the guard fail closed (#4, #5) before any real use — everything else can follow.
