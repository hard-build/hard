# Repository instructions

## Context

- The current Go module and implementation live in `hard/`.
- Root-level `README.md`, `MEMORY.md`, `hard.h`, `format/`, and
  `environment/` belong to the same current project.
- Read `MEMORY.md` before project work; it is the durable implementation and
  decision record.

## Working rules

- Read every file completely before editing it.
- For a task requiring more than one edit, present a file/change/check plan
  and wait for confirmation before modifying files.
- Make only the requested change; avoid unrelated refactors and formatting.
- Ask when materially different interpretations would require different work.
- Preserve unrelated and untracked user changes.
- Obtain explicit approval before destructive or external actions.
- Update `MEMORY.md` when requirements, decisions, implementation status,
  dependencies, tests, known gaps, or verification procedures change.
- Update `README.md` when user-facing behavior changes, keeping it English
  and limiting the public commands to `environment`, `format`, `build`,
  `fetch`, `run`, and `test`.

## Verification

Run Go checks from `hard/`:

```bash
gofmt -d *.go
go test ./...
go test -race ./...
go vet ./...
go build -o /tmp/hard-check .
go mod verify
```

Run `git diff --check` from the repository root. For documentation changes,
reread `AGENTS.md`, `MEMORY.md`, and `README.md` completely, validate
local links and current paths, and reread the full diff skeptically.
Do not report completion when required checks have not passed.
