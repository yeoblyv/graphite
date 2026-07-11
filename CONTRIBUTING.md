# Contributing

## Building and testing

Using `make` (Linux/macOS/CI):

```bash
make build   # compile for the host platform
make test    # go test ./...
make vet     # go vet ./...
make fmt     # fail if gofmt would change anything
make lint    # golangci-lint, if installed
```

Using PowerShell (Windows, no extra tools required):

```powershell
./build.ps1 -Target Build
./build.ps1 -Target Test
./build.ps1 -Target Vet
./build.ps1 -Target Fmt
./build.ps1 -Target Lint
```

Both accept cross-compile targets (`build-linux-amd64`,
`-Target linux-amd64`, etc.) — see the README's Building section.

## Before opening a pull request

- `gofmt -l .` reports nothing.
- `go vet ./...` and `golangci-lint run ./...` are clean.
- `go test ./...` passes; new or changed behavior ships with a test, and bug
  fixes ship with a regression test that fails without the fix.
- New or changed exported symbols carry a godoc comment.
- Comments and commit messages are in English.

## Reporting a bug or proposing a change

Open an issue describing the problem or the proposed change before sending a
large pull request, so the approach can be agreed on first.
