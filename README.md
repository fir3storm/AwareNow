# AwareNow

AwareNow is an AwareNow-branded, Gophish-compatible security-awareness
simulation platform. It includes the Go application, web frontend, deployment
examples, operational scripts, and curated email and landing-page templates.

Use it only for authorized security-awareness training, internal testing, and
other engagements for which you have explicit permission.

## Local development

Requirements: Go 1.21 or newer and Node.js with npm.

```bash
git clone https://github.com/fir3storm/AwareNow.git
cd AwareNow
go test ./...
go build

cd web
npm ci
npm run lint
npm run build
```

Run the Go application with your local configuration, then open the admin UI
at `https://localhost:3333`. The first-run administrator password is printed in
the application log.

## Deployment

The `deploy/` directory contains example configuration for a Linux host using
systemd and nginx. `scripts/install.sh` installs the compatible service and
nginx configuration; review every example and replace hostnames, credentials,
and certificate paths before using it in an environment.

The reference deployment uses these domains and ports:

| Role | Hostname | Local port |
| --- | --- | ---: |
| Campaign landing pages | `itsupport.insec.in` | `8082` |
| Administrator console | `admin.itsupport.insec.in` | `3333` |
| nginx HTTPS | public hostnames | `443` |

The deployment examples retain upstream-compatible service identifiers where
the runtime requires them. The reference installation root is `/opt/awarenow`;
review sample domains and host settings before using them in production.

## Template library

AwareNow includes original INSEC templates and vendor template packs under
`templates/insec/` and `templates/vendor/`. Review the provenance and license
files in each vendor directory before redistribution.

To import templates into an authorized local instance, set the API key and
run:

```bash
export GOPHISH_API_KEY="<authorized-instance-api-key>"
python3 scripts/import-templates.py
```

The compatibility environment variable is intentionally named
`GOPHISH_API_KEY` because the application API remains Gophish-compatible.
Additional filtering guidance is in `templates/README.md` and the helper
scripts under `scripts/`.

## Repository layout

- `auth/`, `controllers/`, `models/`, and related directories: Go backend
- `web/`: current frontend application
- `templates/`: server UI and campaign template library
- `deploy/`: systemd and nginx examples
- `scripts/`: installation and template-management helpers
- `docs/`: project design, plans, and audit records

## License

See `LICENSE` and the individual vendor license files for applicable terms.
