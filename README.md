# envsync

`envsync` keeps real dotenv files aligned with their `.env.example` templates
without replacing secrets or evaluating file contents. It is a dependency-free
Go CLI distributed as a single binary.

```text
.env.example                 .env before
# Database                   DATABASE_URL=postgres://secret
DATABASE_URL=postgres://...  OLD_OPTION=true
LOG_LEVEL=info

                             .env after
                             # Database
                             DATABASE_URL=postgres://secret
                             LOG_LEVEL=info

                             # --- envsync: local entries not present in the example ---
                             # envsync:disabled OLD_OPTION
                             # OLD_OPTION=true
                             # envsync:end-disabled OLD_OPTION
```

Existing values always win. Missing keys receive the example default, extra
active keys are reversibly commented out, and the example supplies ordering,
comments, and spacing. If a disabled key later returns to the example, its old
value is restored automatically.

## Install

On macOS, Linux, or FreeBSD, install the latest release with:

```sh
curl -fsSL https://envsync.tabw.dev/install.sh | sh
```

The installer detects your OS and architecture, verifies the release checksum,
and places `envsync` in `~/.local/bin`. Set `ENVSYNC_INSTALL_DIR` to choose
another directory. Windows users can download the appropriate setup executable
from the repository's Releases page. It installs `envsync` for the current user
and adds it to `PATH`; open a new terminal after installation.

To build from source with Go 1.25 or newer:

```sh
git clone git@github.com:theoabw/envsync.git
cd envsync
go build -trimpath -o envsync ./cmd/envsync
```

## Quick start

Run `envsync` in a project containing `.env.example`. If `.env` does not exist,
the example is copied exactly and created with private permissions on Unix.

```sh
envsync                         # sync every pair in the current directory
envsync --dry-run               # preview key-level actions without values
envsync --check                 # exit 1 when files need synchronization
envsync --dir ./services/api    # use another directory
envsync --backup                # keep timestamped copies of changed files
envsync --keep-extra            # leave newly found extra keys active
```

By default, discovery recognizes these pairs:

| Example | Destination |
| --- | --- |
| `.env.example` | `.env` |
| `.env.production.example` | `.env.production` |
| `.env.local.example` | `.env.local` |

Every match is synced. Discovery stays in one directory unless `--recursive`
is used. Recursive mode skips `.git`, `.hg`, `.svn`, `node_modules`, and
`vendor`; use `--no-default-excludes` to include them.

Use quoted, repeatable globs to limit discovery. A pattern without `/` matches
the filename; a pattern containing `/` matches the slash-separated path
relative to `--dir`.

```sh
envsync --recursive --match '.env.production.example'
envsync --match '.env.*.example' --exclude '.env.test.example'
```

Supplying any `--match` replaces the built-in include patterns. For a custom or
single pair, bypass discovery:

```sh
envsync --example config/example.env --env config/runtime.env
envsync --example .env.staging.example  # destination is .env.staging
```

## Merge rules

- Active destination assignments retain their complete value and syntax; a
  changed example default never overwrites them. An empty value is intentional.
- Missing assignments are copied from the example and reported by key name so
  you can review their defaults.
- Extra assignments are placed in reversible `envsync:disabled` comment blocks.
  `--keep-extra` keeps newly encountered extras active in the local section but
  does not reactivate blocks disabled by an earlier run.
- Example comments form the main document. Destination-only comments are kept
  in the local section. Ordinary commented assignments are comments, not stored
  values, and are never automatically activated.
- Duplicate active keys, duplicate managed-disabled keys, malformed lines, and
  unterminated multiline quotes stop the entire preflight before writes begin.
- All selected pairs are validated first and then replaced atomically one file
  at a time. A late I/O error can therefore leave earlier pairs updated; the
  command reports this explicitly. `--backup` provides recovery copies.

Supported syntax includes `KEY=value`, optional `export`, blank values,
whitespace, inline comments, dots/hyphens in key names, quoted `#` and `=`, and
single- or double-quoted values spanning lines. LF, CRLF, and an initial UTF-8
BOM are handled. `envsync` deliberately does not expand variables, escapes,
commands, or shell expressions.

Destination symlinks are refused by default. `--follow-symlink` resolves the
link and atomically replaces its target while leaving the link itself intact.

## Command reference

```text
envsync [options]
envsync --example PATH [--env PATH] [options]

  --dir PATH                 scan/base directory (default .)
  --recursive                scan nested directories
  --match GLOB               include glob; repeatable, replaces defaults
  --exclude GLOB             exclude glob; repeatable
  --no-default-excludes      scan dependency/vendor directories recursively
  --example PATH             exact example file (single-pair mode)
  --env PATH                 destination for --example
  --keep-extra               keep newly found extra assignments active
  --follow-symlink           update a destination symlink target
  --dry-run                  preview without writing
  --check                    exit 1 if changes are required
  --backup                   make timestamped adjacent backups
  --quiet                    suppress normal output
  --color auto|always|never  control ANSI color (default auto)
  --version                  print version
  --help                     show detailed help and examples
```

`NO_COLOR` disables automatic color. Existing values and value-bearing diffs
are never printed.

Exit codes are `0` for success, `1` when `--check` detects drift, and `2` for
usage, parsing, discovery, or I/O errors.

## Development

```sh
go test ./...
go test -race ./...
go vet ./...
```

The parser and reversible marker format are fuzz-tested. The runtime uses only
the Go standard library.

## AI disclosure

This project was developed with assistance from generative AI tools. The
maintainer reviewed, tested, and takes responsibility for the resulting code
and documentation.

## License

MIT
