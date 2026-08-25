# Contributing

Thanks for considering contributing to qr-multi-imgs.

## How to help

- **Report bugs** - open a [GitHub Issue](https://github.com/thousandflowers/qr-multi-imgs/issues/new?template=bug_report.md) with reproduction steps.
- **Suggest features** - open a [Feature Request](https://github.com/thousandflowers/qr-multi-imgs/issues/new?template=feature_request.md).
- **Submit code** - follow the steps below.

## Development setup

```bash
git clone https://github.com/thousandflowers/qr-multi-imgs.git
cd qr-multi-imgs
go build .
go test ./...
```

Requires Go 1.26+.

## Code style

- Run `gofmt` before committing.
- Keep the test suite green: `go test -count=1 ./...`
- Keep `go vet` clean: `go vet ./...`
- New helper functions should include tests.
- Prefer the standard library over new dependencies.

## Pull request process

1. Create a branch from `main`.
2. Make your changes with clear commit messages.
3. Add or update tests.
4. Verify `go build ./... && go vet ./... && go test -count=1 ./...` passes.
5. Open a PR against `main`.

## License

By contributing, you agree that your contributions will be licensed under the [MIT License](LICENSE).
