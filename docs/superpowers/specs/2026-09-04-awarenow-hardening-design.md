# AwareNow Hardening and UI Rebranding Design

Make deployment upgrades safe, limit template cleanup to AwareNow-owned records,
verify backend and frontend in CI, and remove `Gophish` from browser-visible UI.
The outer repository owns nginx and template scripts; `awarenow-source` owns the
server and UI. The browser-facing name is **AwareNow**. Nginx installation will
preserve certificate-managed configuration on upgrades; `cookie_secure` protects
cookies when TLS terminates at nginx. Cleanup validates credentials before any
mutation and deletes only an explicit retired-HailBytes name manifest. An audit
rejects browser-served legacy-name text. CI retains Go 1.21--1.23 and adds
frontend dependency installation, lint, typecheck, and production build.
