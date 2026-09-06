# Repository Guidelines

## Project Structure & Module Organization

AwareNow is a security-awareness simulation platform. The legacy Go service
lives at the repository root: `controllers/` contains HTTP handlers,
`models/` persistence types, and packages such as `auth/`, `mailer/`,
`middleware/`, and `worker/` provide shared behavior. Server-rendered views
and campaign content are in `templates/`; third-party template packs remain
under `templates/vendor/` with their provenance files intact.

The current React frontend is in `web/src/`. The TypeScript management-plane
service is isolated in `control-plane/src/`, with its Prisma schema in
`control-plane/prisma/`. Deployment examples, operational helpers, and design
records live in `deploy/`, `scripts/`, and `docs/` respectively.

## Build, Test, and Development Commands

Run Go checks from the repository root:

```sh
go test ./...
go build
```

Run frontend checks from `web/`:

```sh
npm ci
npm run lint
npm run test:run
npm run build
```

Use `npm run dev` in `web/` for the Vite development server. For the control
plane, run `npm ci && npm run test:run && npm run typecheck && npm run build`
from `control-plane/`; use `npm run dev` to watch `src/server.ts`.

The documented `go test ./...` may currently fail in the legacy Goose/SQLite
dependency before application tests run. Report that failure separately from
changes you make unless your work addresses it.

## Coding Style & Naming Conventions

Format Go code with `gofmt`; use idiomatic exported `PascalCase` names and
unexported `camelCase` names. Keep Go tests beside their package as
`*_test.go`. Follow the existing TypeScript style: `camelCase` variables and
functions, `PascalCase` React components, and co-located `*.test.ts(x)` tests.
Run `web`'s `npm run lint` before submitting UI changes.

## Testing Guidelines

Add focused regression coverage with each behavior change. Go tests use the
standard test runner; the web and control-plane projects use Vitest. Test
public routes, authorization boundaries, and error paths when changing APIs.
Do not commit credentials: use checked-in `.env.example` files and local
environment variables.

## Commit & Pull Request Guidelines

Recent commits follow concise Conventional Commit subjects, for example
`feat: add reported-message review API` or `fix: resolve API compile errors`.
Keep each commit scoped. Pull requests should explain the user-visible change,
list validation performed, link related issues or design docs, and include
screenshots for UI changes. Report security vulnerabilities privately as
described in `SECURITY.md`, not in public issues.
