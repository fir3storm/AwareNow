# Safe local validation

Run frontend checks from the repository root:

```sh
cd web
npm ci
npm run lint
npm run build
```

Run the Go test suite from the repository root:

```sh
go test ./...
```

No deployment, campaign setup, mail configuration, or credentials are required for these checks.

## Known legacy Go test issue

At the time of writing, `go test ./...` fails while compiling the legacy
`bitbucket.org/liamstask/goose/lib/goose` dependency because it references the
removed `sqlite3.Error` symbol:

```text
bitbucket.org/liamstask/goose/lib/goose/dialect.go:119:15: undefined: sqlite3.Error
```

The failure affects the root package and dependent `controllers` and `imap`
packages before their tests can run. It is unrelated to the frontend checks.
