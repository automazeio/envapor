# Changelog

All notable changes to Envapor are documented here.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.1.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [Unreleased]

## [0.1.0] - 2026-07-15

### Added

- Initial release. Transparent encryption of `.env` values in Git via clean/smudge
  filters, a pre-commit guard, AES-256-GCM with HKDF-derived subkeys, and
  deterministic, context-bound `ENC[v2:...]` tokens for readable diffs and clean
  merges. Includes `keygen`, `keys`, `init`, `status`, `doctor`, `verify`,
  `encrypt`, `decrypt`, `migrate`, and `install-git-shim` commands, a `git envapor` Git
  subcommand shim, a first-party GitHub Action, install scripts for
  macOS/Linux/Windows, and an agent skill.
- `envapor verify` command that checks the Git-stored (index) content of managed
  `.env` files and exits non-zero if any non-`PUBLIC` value is still plaintext,
  for use in CI and pre-push hooks.

[Unreleased]: https://github.com/automazeio/envapor/compare/v0.1.0...HEAD
[0.1.0]: https://github.com/automazeio/envapor/releases/tag/v0.1.0
