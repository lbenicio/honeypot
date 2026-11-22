# Contributing

Bug reports, documentation improvements, and focused code changes are welcome.
For a large change, open an issue first to discuss the approach.

## Development

The project uses the Go version declared in `go.mod` and builds with the Go
toolchain pinned there. Before submitting a change, format Go files and run:

```sh
gofmt -w src/*.go
make build
make test
```

If you change a GitHub Actions workflow, validate it with `actionlint` when
available. If you change dependencies, update both `go.mod` and `go.sum`.
The build target writes the executable to `dist/fakessh`.

## Pull requests

- Describe the problem and the change, and link a related issue when there is
  one.
- Include the commands you ran and their results.
- Keep the change focused and update documentation when behavior or setup
  changes.
- Add a concise entry under `[Unreleased]` in `CHANGELOG.md` for user-visible
  changes.
- Do not commit credentials, tokens, real captured passwords, or sensitive
  honeypot logs.

## License

By contributing, you agree that your contributions are distributed under the
project's GNU General Public License version 3, as described in `LICENSE.txt`.
