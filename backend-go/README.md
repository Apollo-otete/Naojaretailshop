# Naoja Retail Shop — Go API

Phase 2 of the reform described in `Naoja_Retail_Shop_Architecture_and_Reform_Decisions.pdf`.
This is **not the live shop**: `../backend` (Express + MongoDB/Mongoose) keeps serving while this
service grows. Nothing here is deployed and nothing here talks to production.

§19 step 5 — *"Go service skeleton, configuration, logging, health checks, PostgreSQL
connection, migrations, and test harness"* — is complete against its Phase 2 exit criterion:
*"Deployable API skeleton with automated checks."*

Work is now on **step 6** (*"Build identity/RBAC and catalogue as the first vertical slices, with
tests and a usable frontend integration"*), whose Phase 3 exit criterion is *"Role lifecycle and
authorization tests pass"*. The identity half is complete — the schema, the Argon2id and session
rules, the RBAC evaluator and its ownership conditions, the middleware chain that applies them, and
the contract's five `/auth` operations on top of it. The catalogue half has now been built the same
way and its tests are in: domain rules in `internal/catalogue`, store/service/transport wired
through the ports, migrations 0005, and a handler suite that pins the mounted operation set to the
contract's. The shopping half has now been built the same way too: the Commerce context's rules in
`internal/shopping`, the driving adapter in `internal/httpshopping` — read, add, update, remove and
clear a cart, with the guest session token the public operations run on — and a handler suite that
pins the mounted operation set to the contract's. The ordering half has now been tested the same way:
the status vocabulary in `internal/ordering`, and the handler suites for the placement, read, list and
status operations. The role-boundary suite (matrix B1–B13, B15) passes against PostgreSQL, and
B14/B16/B17 stay where the slices they need are not built yet. See *Deliberate state* below.

Both criteria are checkable here:

```
make check             # gofmt -l clean, go vet clean, unit tests pass
make test-integration  # migrations applied and fully rolled back against PostgreSQL 16
make run               # /healthz 200, /readyz reports postgresql, 404/405 as contract envelopes
```

---

## Quickstart

```bash
cd backend-go
make db-up                     # PostgreSQL 16 on :5433 (+ the naoja_test scratch database)
cp .env.example .env           # .env is gitignored and must stay that way
make migrate-up
make run                       # :8080
curl -s localhost:8080/readyz
```

`make help` lists every target. The two that matter for a review are `check` (what CI runs) and
`test-integration` (destructive; it only connects to a database whose name ends in `_test`).

Once it is running, the identity slice is exercisable by hand:

```bash
curl -si -X POST localhost:8080/api/v1/auth/register -H 'Content-Type: application/json' \
  -d '{"full_name":"Achieng Otieno","email":"a@mail.test","phone":"0712345678",
       "password":"correct horse battery staple"}'
# 201, a Set-Cookie carrying the session, and a Session body: the roles and 20 effective
# permissions the seeded customer role grants, the 15 of them that are ownership-scoped, and the
# csrf_token every unsafe method must then send in X-CSRF-Token.
```

**Toolchain:** Go 1.22.2, pinned via `GOTOOLCHAIN=local` in the Makefile. `go get` will otherwise
rewrite the `go` directive to a version this repository does not build against — a dependency
already demands 1.25 and that is not a decision to take implicitly.

---

## Layout, and why it is cut this way

```
cmd/api/          composition root: config, logger, pool, migrator, engine, signal handling
cmd/migrate/      operational CLI — up | down [n] | down -all | status | force
internal/config/  environment → typed Config, plus the rules that refuse an unsafe startup
internal/observability/  slog JSON, request ids on the context
internal/apierr/  the single vocabulary of failures — no HTTP or database imports
internal/httpjson/   the only code that writes a response body
internal/httpmw/  request id, access log, recovery, CORS, session read, CSRF, authorisation
internal/port/    the interfaces (Checker, Authorizer) adapters implement — imports no Gin
internal/httpauth/  the /auth routes — request reading and error mapping, no rules
internal/authz/   the RBAC evaluator and the attribute-condition registry (ADR-024)
internal/database/    pgx pool + golang-migrate over embedded SQL
internal/server/  engine assembly and the health endpoints
migrations/sql/   versioned up/down pairs, embedded into the binary
contracts/../     the OpenAPI contract this service is written against
docs/../          the Phase 1 decisions (ADRs, permission matrix, domain model)
```

§8 of the reform document is the reason for `internal/port`: *"A PostgreSQL adapter implements
that port. Gin only handles HTTP transport and must not contain business rules."* The test is
mechanical — if a package under `internal/port` ever needs to import Gin, that sentence has been
broken.

Modular monolith, not microservices (§7). One deployable, bounded contexts separated by PostgreSQL
schemas so a later split is a refactor rather than a rewrite.

---

## What is enforced by code, not by discipline

These are the reform's requirements that a reviewer can check by reading a test, rather than by
trusting a checklist.

| Requirement | Where it lives | How it fails |
|---|---|---|
| Secrets never in logs (§10) | `config.Database.RedactedURL` | `TestRedactedURLNeverCarriesThePassword` |
| No `Access-Control-Allow-Origin: *` with credentials (ADR-019) | `config.Validate` | the process refuses to start |
| Deny by default (§10) | `authz.RBAC`, `httpmw.Authorize` | `TestRequireRefusesAnAnonymousRequest`, `TestAnonymousRequestIsRefusedBeforeAnythingIsNarrowed` |
| A revoked or de-permissioned caller fails on the **next request** (B11, B12, ADR-019) | `identity.Service.Authenticate` + `httpmw.Authenticate`, both read per request with no cache | `TestOneEvaluatorAnswersForEveryCallerWithoutRememberingAny`, `TestRemovingAPermissionFromARoleRemovesItFromItsMembers` |
| `[own]` permissions are narrowed by ownership (matrix §2, ADR-024) | `identity.permissions.conditions` + `authz.conditions` | `TestOwnershipNarrowsAPermissionTheCallerHolds`, `TestOwnershipIsDeclaredForEveryKeyTheProseMarksOwn`, `TestTheCatalogueRefusesAConditionNothingImplements` |
| A dead session cannot make a public route fail | `httpmw.Authenticate` clears and continues | `TestADeadSessionContinuesAnonymousAndClearsTheCookie`, `TestHealthzNeverReadsASession` |
| One error shape (contract `Error`) | `apierr` + `httpjson.Write` | `TestWriteRendersTheContractEnvelopeWithTheCorrelationID` |
| A 500 leaks nothing (§10) | `apierr.AsFailure` default | `TestInternalCauseIsNeverSerialized` |
| Correlation id on every line (§10) | `httpmw.RequestID`, `observability.From` | `TestChainLogsOneAccessRecordPerRequestWithItsCorrelationID` |
| No customer identifiers in logs | `httpmw.Access` logs `c.FullPath()` | `TestAccessLogRecordsTheRoutePatternNotTheRawPath` |
| A caller cannot forge logs or headers | `httpmw.validRequestID` | `TestMalformedInboundRequestIDsAreReplaced` |
| A client cannot choose its own recorded IP | `SetTrustedProxies(nil)` | `TestForwardedForCannotChooseTheClientIP` |
| Every schema change is reversible (§16) | `migrator.DownAll`, paired `.down.sql` | `TestMigrationsRunUpAndBackDownAgainstPostgres`, `TestEveryUpHasItsDownAndViceVersa` |
| Money is integer cents (ADR-021) | `migrations/sql` CHECKs | `TestMoneyColumnsAreIntegers` |
| Unratified business terms cannot read as approved | `system_settings.ratified` | `TestUnratifiedSeedsAreMarkedAsPlaceholders`, `TestMigrationsRunUpAndBackDownAgainstPostgres` |
| A failed login never says whether the account exists (matrix C1) | `identity.ErrInvalidCredentials` covers absence, wrong password, suspension and deactivation | `TestLoginFailuresDoNotDiscloseTheAccountState` |
| A password reaches neither response body nor log (§10) | `httpauth` renders `Session`, never the input; `decode` refuses to echo a parse error | `TestThePasswordNeverComesBack`, `TestRegisterRefusesAJunkBodyWithoutEchoingIt` |
| Logout is idempotent and leaks no session state (contract `/auth/logout`) | `httpauth.logout` answers 204 with or without a principal | `TestLogoutWithADeadOrAbsentCookieStillSucceeds` |
| The cookie one route issues is the value the next route reads | `httpcookie` + `httpmw.Authenticate` + `httpauth.Mount` | `TestTheSessionOneRouteIssuesIsTheSessionTheNextRouteReads` |
| Only the operations the contract documents are routed | `httpauth.Mount` | `TestTheMountedOperationsAreExactlyTheContractedOnes` |
| Only the operations the contract documents are routed | `httpshopping.Mount` | `TestTheMountedOperationsAreExactlyTheContractedOnes` |
| The public auth operations are throttled per client address | `httpauth.RateLimit`, `httpauth.RateLimiter` | `TestLoginAnswers429WhenThePerAddressLimitIsExceeded`, `TestRegisterIsThrottledPerAddress` |

---

## Deliberate state: identity is reachable, nothing else is

Authorisation is real and it is now reachable: `cmd/api` wires `authz.RBAC` over the seeded
catalogue, every route is behind `httpmw.Authenticate` and `httpmw.CSRF`, and the contract's five
`/auth` operations answer on `/api/v1`. Register or log in against a running service and the cookie
that comes back works on every protected route, so the whole authentication path can be exercised
end to end rather than only in unit tests.

An allow-stub would have made the skeleton "work" and is exactly how an unauthenticated endpoint
ships by accident; DenyAll was its safe version, and it was replaced rather than relaxed.

Also not present, on purpose:

- **Every business endpoint from the contract.** Checkout and payments are the next slices — quotes,
  capture, refunds — and the `/auth/register` promise that "the guest cart held by this browser is
  merged into the new account's cart" is not. The cart is built — read, add, update, remove and clear
  under `/api/v1/cart` — and the order operations the contract describes beyond those (placement, read,
  list, status) are under test in `internal/ordering`.
- **The breached-password-list check** registration's contract text asks for. No ADR decides which
  provider answers it, and guessing one would put a third-party call inside account creation.
- **A request body size limit.** Every `/auth` handler decodes the body it is given, so an oversized
  one is a real cost. `MaxMultipartHeaderBytes` covers headers and nothing covers this.
- `pgx`-generated queries (`sqlc` arrives with the first real repository), the Daraja adapter, and
  background jobs.

Migration-on-boot is **off** by default (`AUTO_MIGRATE=false`); schema changes belong to the deploy
step, where a human can take a backup first.

---

## Health endpoints

Neither is under `/api/v1`: they are an operations surface, and versioning them implies a
compatibility promise to clients that should not be calling them.

- `GET /healthz` — process liveness only, no dependency touched. An orchestrator restarts a pod
  for a live failure, and restarting the API because the database is slow replaces a temporary
  outage with a stampede.
- `GET /readyz` — every dependency checked **concurrently** and reported individually, because
  "unhealthy" tells an operator nothing. Failure detail is present in development and suppressed
  in production: a driver message can name a host and a user.

Adding a dependency means appending a `port.Checker` to `server.Deps.Checkers`. Nothing else.

---

## Migrations

`make migrate-up | migrate-down N=2 | migrate-status | migrate-force V=2 YES=1 | migrate-down-all`.

SQL lives in `migrations/sql` and is embedded into both binaries, so a container carries its own
schema history and `cmd/migrate` needs nothing but `DATABASE_URL`. golang-migrate is used as a
library rather than a CLI download so there is one version of the migration logic, in `go.sum`.

Two rules are worth knowing before adding a file:

- **Pairs, always.** An `.up.sql` without a `.down.sql` is a migration that cannot be undone, and
  a failed up leaves the database dirty with no route out but `force`.
- **Seeded values declare their provenance.** `administration.system_settings` carries
  `ratified` and `description`. `delivery_standard_fee_cents` and `free_delivery_threshold_cents`
  are seeded `ratified = false` and labelled PLACEHOLDER because **ADR-018 is still open** — the
  numbers are what the prototype UI displayed, not what the business approved.

---

## Reading order for a review

1. `docs/phase-0-inventory.md` — what exists today and what is broken about it.
2. `docs/phase-1-actor-permission-matrix.md` — approved; the source of every permission key.
3. `docs/phase-1-domain-model-erd.md` — bounded contexts, ERD, and the transaction boundaries T-01…T-10.
4. `docs/adr/ADR-015-023-open-item-resolutions.md` — ADR-015…017, 019…023 Accepted;
   **ADR-018 (delivery terms) is still open**. ADR-001…014 are the reform document's own §7
   decisions and have no separate file.
6. `docs/adr/ADR-024-attribute-conditions-on-role-permissions.md` — **Proposed**; the second gate
   on top of the permission check, and the two questions it leaves open.
5. `contracts/openapi.yaml` — the API contract Phase 4+ implements.
