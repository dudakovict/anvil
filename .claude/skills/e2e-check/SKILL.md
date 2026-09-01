---
name: e2e-check
description: Spin up the docker compose stack and run the full end-to-end request matrix — health, auth, RBAC, validation, rate limiting, security headers, graceful shutdown. Use to verify the whole service works after changes.
---

# End-to-end check

Resets the compose stack to a clean database, runs the request matrix, then
tears down. Requires docker. Adapt the matrix when endpoints change.

## 1. Fresh stack

```bash
docker compose down -v
docker compose up --build -d
for i in $(seq 1 25); do curl -sf localhost:8080/readyz >/dev/null && break; sleep 1; done
```

## 2. Matrix

Run as a script **from a file** (not stdin — `docker compose exec -T` swallows
stdin the script is read from). Expected status per check:

| Check | Expect |
|---|---|
| `GET /healthz`, `/readyz`, `/swagger/index.html` | 200 |
| unknown route / wrong method | 404 / 405 |
| security headers on any response (`nosniff`, `X-Frame-Options`, CSP, `no-store`) | present |
| CORS preflight from unknown origin (when `HTTP_CORS_ALLOWED_ORIGINS` unset) | no `Access-Control-Allow-Origin` |
| register user | 201 |
| duplicate email | 409 |
| invalid body (bad email/short password) | 400 with `fields` list |
| unknown JSON field | 400 |
| valid JSON body over `HTTP_MAX_BODY_BYTES` (padding must be inside a JSON string — raw garbage fails JSON parsing first and returns 400) | 413 |
| login with wrong password / unknown email | 401 (identical responses) |
| login ok | 200 with `access_token`, `token_type`, `expires_in` |
| protected route without / with garbage token | 401 |
| `GET /me` with token | 200 |
| user role: `GET /users` | 403 |
| user role: other account get/update/delete | 403 |
| user role: own account get/update | 200 |
| promote via `docker compose exec -T postgres psql -U app -d app -c "UPDATE users SET role='admin' WHERE id=1;" </dev/null`, re-login | — |
| admin: list with `?page/?limit/?sort`, other account | 200 |
| `?sort=<unknown>` / `?page=0` / `?limit=101` | 400 |
| missing id / non-numeric id | 404 / 400 |
| admin delete, then get | 204, then 404 |
| 12 rapid logins (run LAST — the limit is per-IP and would poison earlier logins) | mix of 401 then 429 |

## 3. Observability

- Traces landed: `curl -s "http://localhost:3000/api/datasources/proxy/uid/tempo/api/search?limit=5"` returns recent `golang-template` traces
- Logs: `docker compose logs app` — JSON lines with `trace_id`/`request_id`

## 4. Graceful shutdown

```bash
docker compose stop app
docker inspect <app-container> --format '{{.State.ExitCode}}'   # expect 0
docker compose logs app | tail -2   # expect "server stopped gracefully"
```

## 5. Teardown

```bash
docker compose down -v
```

Report results as a pass/fail list; any FAIL is a finding to investigate, not
to paper over.
