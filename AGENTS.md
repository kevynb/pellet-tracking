# Pellet Tracking Agent Guide

## Repository conventions
- Use `gofmt` (automatically handled by `goimports` or `gofmt`) on all Go files you modify.
- Prefer `make` targets for common workflows. Useful commands:
  - `make tools` installs development helpers such as `mockgen`.
  - `make build` compiles the release binary into `bin/pellets`.
  - `make run` builds and runs the HTTP server locally.
  - `make test` runs all unit and integration tests (`go test ./...`).
  - `make e2e` executes the end-to-end suite in `test/e2e` against the compiled binary.
  - `make lint` executes `golangci-lint` with the repository configuration.
  - `make docker` builds the minimal distroless container image.

## Go version
- The project targets Go `1.25.3`. Ensure your local toolchain matches (the Dockerfile uses the same version).

## Testing guidelines
When adding or updating Go tests, follow these mandatory rules:
- Colocate unit tests beside their implementation in `*_test.go` files.
- Separate pure logic unit tests from integration or datastore tests.
- Use table-driven tests. Each tested function should have exactly one `TestFunctionName` containing the table named `tcs`, with individual cases referenced as `tc`.
- Define `params` and `want` structs in every table to describe inputs and expected outputs.
- Call `t.Parallel()` at the beginning of the test and inside each case loop.
- Use `github.com/stretchr/testify/assert` for all assertions and `require` only to guard setup steps that could panic. Include the test case name in assertion messages.
- Generate mocks with `go.uber.org/mock/mockgen` and store them under a `mock/` subdirectory within the package being tested.
- Favor equality assertions over length-only checks and avoid trivial assertions.

End-to-end tests live in `test/e2e` and should exercise happy paths via the compiled binary (not the Docker image). They can match HTML loosely to remain resilient to visual tweaks.

## Architecture overview
- `cmd/app/main.go`: Application entrypoint, wiring configuration, datastore, HTTP/TSnet servers.
- `internal/config`: Environment-driven configuration loader.
- `internal/store`: JSON persistence layer with backup rotation and concurrency safety.
- `internal/core`: Domain models, business operations, money utilities, and statistics.
- `internal/http`: REST API handlers, middlewares, and HTML view templates.
- `internal/tsnet`: Optional Tailscale listener integration.
- `web`: Embedded static assets (CSS/JS) and Go templates.
- `test/e2e`: End-to-end Go tests launching the compiled binary and verifying API/UI flows.

## Additional notes
- Run `go test ./...` before committing and ensure `make lint` passes when modifying Go code.
- Keep JSON responses and HTML templates localized in French where applicable.
- Update this guide if new conventions or tooling requirements are introduced.
