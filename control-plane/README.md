# AwareNow control plane

This directory contains the TypeScript management-plane foundation for
authorized security-awareness simulations. It currently supports local build
and test validation only; it does not provision infrastructure, send mail, or
run campaigns.

## Local validation

Use Node.js 24 or later.

```powershell
npm ci
npm run test:run
npm run typecheck
npm run build
```

`PORT` defaults to `3001` and must be an integer from `1` through `65535`.
`DATABASE_URL`, `CONTROL_PLANE_BASE_URL`, and `AWARENOW_CONTROL_TOKEN` are
optional at this stage; if supplied, their shape is validated without being
logged. The checked-in `.env.example` contains placeholders only—do not add a
real secret to the repository.

## Deliberately deferred engine routing

No Go route is mounted for `/api/v1/control` in this increment. The legacy
engine's campaign operations are user-scoped, so route registration remains
deferred until each isolated engine has an explicit, managed control-owner
identity. This prevents a control token from relying on an implicit legacy
user ID or accessing an unintended owner's campaigns.
