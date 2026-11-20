# AGENTS.md

Go CLI that strictly validates NZ Electricity Authority EIEP files. Stdlib only, Go 1.27.

## Layout

- `cmd/eiep-validate` — CLI: flags, output, colour.
- `internal/eiep` — validation library.
  - `validate.go` — `Format`/`Layout`/`File` types, `formats` registry keyed by HDR file type.
  - `field.go` — `Field` and `Kind` definitions and per-field checks.
  - `eiep13a.go`, `eiep14a.go`, `eiep14v1.go` — one file per format; add new formats the same way and register in `formats`.
  - `json.go` — JSON input (EIEP14A).
  - `plans.go` — `--plans` output.
  - `testdata/` — sample JSON files.

## Commands

```sh
make test   # go vet + go test
go build ./cmd/eiep-validate
```

## Conventions

- Follow the spec exactly; field definitions mirror the spec's data inputs tables.
- Keep code and comments minimal. Add tests alongside each format.
- `data/` and `dist/` are gitignored; don't commit sample data files.
