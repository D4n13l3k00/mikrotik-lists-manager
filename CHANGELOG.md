# Changelog

All notable changes to this project will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.1.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [v1.5.0] - 2026-09-14

### Added
- **Router Profiles (Multi-Router Support)**:
  - Added support for `profiles:` and `default_profile` in the YAML configuration.
  - Added global `--profile / -P` persistent flag and `MT_PROFILE` environment variable.
  - Parameter precedence: CLI flag > environment variable > selected profile > global config > defaults.
  - Updated `config show` to display configuration path, active profile, and available profiles.
- **Global Config Auto-Discovery**:
  - Automatically searches for configuration files in:
    1. Explicit `--config` path
    2. Current directory (`./.mikrotik-lists-manager.yaml`)
    3. System user config directory (`~/.config/mikrotik-lists-manager/config.yaml` on Linux/macOS, `%APPDATA%\mikrotik-lists-manager\config.yaml` on Windows)
    4. User home directory (`~/.mikrotik-lists-manager.yaml`)
  - Added `config init --global / -g` flag to initialize configuration directly into the user's global config directory.
- **Machine-Readable JSON Output (`--json`)**:
  - Added `--json` flag to `list` command for structured list summaries (`[]ListSummaryDTO`) or entries (`[]EntryDTO` when used with `-e`).
  - Added `--json` flag to `find` command outputting structured match objects with match types (`exact` vs `subnet`).
  - Added `--json` flag to `info` command outputting detailed router hardware and RouterOS info (`RouterInfoDTO`).
- **Subnet & CIDR Containment in `find`**:
  - Implemented multi-directional subnet matching using `net/netip`:
    - Needle IP in router CIDR
    - Router CIDR in needle CIDR
    - Needle CIDR in router CIDR
    - Exact match with canonical address normalization
  - Exact matches highlighted in green, subnet matches tagged with `(subnet)`.
- **Bulk Delete Safety Protection in `sync`**:
  - Added safety guard when a synchronization diff would delete > 50 entries or > 50% of the entries on the router.
  - Prompts for interactive confirmation using `huh.NewConfirm` in terminal sessions.
  - Added `--force / -y` flag to bypass confirmation in automation scripts / CI.
  - In non-interactive environments without `--force`, sync safely aborts with an actionable error.
- **Parallelism & Concurrency Upgrades**:
  - Sequential list synchronization in `sync` across `-l list1,list2` to eliminate terminal output and progress-bar interleaving while keeping fast concurrent API execution per-list.
  - Parallelized `enable` and `disable` with `--concurrency / -c` flag (default: 5) and progress bar for $\ge 10$ entries.
  - Parallelized list renaming in `rename` with `--concurrency / -c` flag and progress bar.
  - Bounded concurrency in `fetch` with `--concurrency / -c` (default: 6) using `errgroup.SetLimit`.
  - Multi-ASN queries in RIPE STAT provider parallelized with mutex-protected deduplication.
- **Network Resilience & Custom User-Agent**:
  - Added HTTP 429 Too Many Requests and 5xx automatic retry mechanism in `fetcher` with exponential backoff, jitter, and `Retry-After` header parsing.
  - Added custom `User-Agent: mikrotik-lists-manager/<version> (+https://github.com/D4n13l3k00/mikrotik-lists-manager)` to all HTTP requests across providers and MikroTik REST API.
- **Natural IP/CIDR Sorting**:
  - Implemented natural numerical octet sorting for `list -e` (`compareAddresses`) so `10.0.0.2` sorts before `10.0.0.10`, IPv4 before IPv6, and subnets by prefix length.
- **CI/CD & Testing**:
  - Added `.github/workflows/ci.yml` matrix workflow testing on Ubuntu, Windows, and macOS with `go vet`, `go test -v -race`, and binary compilation.
  - Comprehensive unit test suites added for `internal/mikrotik`, `internal/config`, `internal/cli`, `internal/output`, `internal/parser`, `internal/syncer`, and `internal/optimizer`.

### Fixed
- **Clean Stdout / Stderr Separation**:
  - `export`: Colored headers redirected to `stderr` when exporting to stdout, allowing clean pipe redirection (`export > list.lst`).
  - `optimize`: Operational logs and statistics redirected to `stderr` when outputting to stdout (`optimize file.lst > out.lst`).
  - Fixed `optimize` to output the original list to stdout when no changes were needed.
- **Disabled State Preservation**:
  - Fixed `export` dropping disabled state: now outputs `!` prefix in `native` format and `disabled=yes` in `mikrotik` format.
  - Fixed parser regex to parse `disabled=(yes|true)` from RouterOS RSC files.
- **Uniform Address Normalization**:
  - Fixed direct string comparisons in `append`, `remove`, and `toggle`. Host IPs normalized to `/32` (v4) or `/128` (v6), zeroing host bits on network CIDRs.
- **Deduplication in `syncer.Diff`**:
  - Prevented duplicate change generation when the desired list contains duplicate addresses, taking the last entry's attributes and issuing warnings.

---

## [v1.4.0] - 2026-09-08
- Added `rename` command to rename address-lists on router.
- Added `backup` command to dump all static address-lists into individual files.
- Added `find` command to search for IP or CIDR across all lists.
- Added `info` command to display router hardware and RouterOS info banner.
- Added `completion` command to generate shell completions (bash, zsh, fish, powershell).
- Added multi-list support (`-l a,b` or `-l a -l b`) across all router commands.
- Added interactive password prompt via terminal when `-p` is omitted.

## [v1.3.0] - 2026-09-07
- Added `enable` and `disable` commands for toggling addresses or whole lists.
- Added `append` and `remove` commands for selective address list manipulation without full sync.
- Added `export` command with support for native and mikrotik RSC export formats.

## [v1.2.0] - 2026-09-06
- Added `fetch` command to download CIDR ranges from major cloud and service providers (Cloudflare, Telegram, GitHub, Oracle, AWS, Google, etc.).
- Added `optimize` command to deduplicate entries and remove redundant subnets.
- Added progress bar and verbose flag support.

## [v1.1.0] - 2026-09-05
- Added `list` command to view all address-lists on the router with entry counts.

## [v1.0.0] - 2026-09-04
- Initial release with `sync` command supporting native and mikrotik formats.
