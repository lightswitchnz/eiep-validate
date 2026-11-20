# eiep-validate

Validates NZ Electricity Authority EIEP files strictly against their specs.

Supported formats:

- EIEP13A (ICPCONS) 1.1–1.3
- EIEP14A (PRCSCHD) 1.0, CSV and JSON, plus the original EIEP14 v1

## Build

```sh
go build ./cmd/eiep-validate
make dist   # cross-compiled binaries in dist/
make test
```

## Usage

```sh
eiep-validate [--json] [--all] [--strict] [--plans] [FILE...]
```

- `--json` print every finding as JSON
- `--all` print every finding instead of summarising repeats
- `--strict` also warn about values longer than the spec allows and plans sharing a description
- `--plans` list each EIEP14A plan with the attributes that apply to it

With no FILE, opens a web UI in the browser.

Exit status: 0 valid (warnings allowed), 1 findings, 2 usage or read error.

## License

MPL-2.0
