# ADR-015 → ADR-023 — Resolution of §18 Open Items

Resolves the six questions that block the domain model and ERD (§19 step 3), plus three
consequential decisions that fell out of ratification. Written to the Appendix B template.

## Ratification record — 2026-11-05

The product owner reviewed the items that carried a **RATIFY** flag and answered them:

| ADR | Question put | Answer | Effect |
|---|---|---|---|
| 016 | May Admin 2 set product prices? | **Ratified as written** | Admin 2 holds `catalogue.products.update_price`; reason, audit and prospective-only stand. No request/approval queue. |
| 020 | Are refunds in launch scope, disbursed manually? | **Ratified as written** | `payments.refund.request` / `payments.refund` split holds; no auto-restock; no B2C at launch. |
| 022 | What do analytics call revenue? | **Settlement minus executed refunds** | Replaces `payment_status='paid' OR status='delivered'`. Recorded as ADR-022. |
| 023 | One stock location or several? | **One location** | `inventory.product_id UNIQUE` is a commitment, not a default. Recorded as ADR-023. |

**ADR-015, 016, 017, 019, 020, 021, 022 and 023 are Accepted.** **ADR-018 remains Proposed**
because its zone coverage and fee amounts are commercial terms only Naoja Ventures can supply
— the three inputs needed are listed in its **Status** block. Nothing in Phase 2 or Phase 3
depends on ADR-018; Phase 5 does.


---

# ADR-015: Order lifecycle and permitted transitions

- **Status:** Accepted
- **Date:** 2026-11-05

## Context

§18 leaves the order status set undefined. Today four different sources disagree and none
of them enforce anything:

- `naoja_schema.sql:59` comments `pending, confirmed, shipped, delivered, cancelled` — but
  that file is a generated artifact and is not the live schema (§7.2 of the inventory).
- `initPg.js` defaults `status` to `'pending'` with no CHECK constraint.
- `mpesa.js:150` sets `'confirmed'` when a payment succeeds and `status` was `'pending'`.
- `PUT /api/orders/:id` (`index.js:346`) writes **any string** an admin submits.
- The admin UI offers `pending / confirmed / shipped / delivered / cancelled` in a `<select>`.

Two structural problems: order state and payment state are conflated (revenue is currently
counted as `payment_status='paid' OR status='delivered'`, `index.js:530`), and a single
linear status cannot express the shop's two fulfilment paths — store pickup and local
delivery — which genuinely diverge after preparation.

## Decision

**Order status and payment status are two independent state machines on the order, each with
an enforced transition table and an append-only history.**

Order status:

| Status | Meaning | Applies to |
|---|---|---|
| `pending` | Created, awaiting verified payment | both |
| `confirmed` | Payment verified, awaiting preparation | both |
| `processing` | Staff are picking/assembling/testing | both |
| `ready` | Prepared and awaiting customer collection | **pickup only** |
| `dispatched` | Left the store, in transit | **delivery only** |
| `delivered` | Completed — collected or received | both |
| `cancelled` | Terminated before leaving the store | both |
| `returned` | Goods back with the store after delivery | both |
| `refunded` | Money returned; terminal | both |
| `expired` | Payment window elapsed with no settlement; terminal | both |

Permitted transitions — nothing not listed is allowed:

```
pending     → confirmed | cancelled | expired
confirmed   → processing | cancelled
processing  → ready | dispatched | cancelled
ready       → delivered | cancelled
dispatched  → delivered | returned
delivered   → returned
returned    → refunded | delivered
cancelled   → refunded
expired     → confirmed          (late settlement reconciled by callback)
refunded    → (terminal)
```

Rules that make this safe:

1. `shipped` is **retired** in favour of `dispatched`. Two names for one state is how the
   revenue rule became ambiguous.
2. `ready` and `dispatched` are mutually exclusive, chosen by the order's fulfilment method.
   An order with `fulfilment_method='pickup'` cannot reach `dispatched`, and vice versa.
3. `expired` exists because §12 step 6 requires "failed, cancelled, or timed-out payments
   must follow explicit expiry/release rules." Today nothing ever expires.
4. `expired → confirmed` is permitted so a genuinely late-but-successful Daraja callback
   still settles the order rather than orphaning paid money. This is the case the current
   phone-suffix fallback was clumsily trying to handle.
5. `returned → delivered` allows a mis-marked return to be reversed without inventing a
   status.
6. `refunded` is only reachable from `returned` or `cancelled`, never from `delivered`
   directly — goods must be accounted for before money moves. See ADR-020.
7. `cancelled` is not reachable from `dispatched`. Once goods have left the store the path
   is `dispatched → delivered → returned → refunded`, or a `returned` entered directly with
   a recorded reason.
8. Transition authority: `ordering.orders.process` (Admin 2) may walk the normal path only.
   `ordering.orders.status.override` (Admin 1, Super Admin) is required for any non-adjacent
   move, and always records a reason.
9. Every transition writes `order_status_history` — actor, previous, next, reason,
   timestamp. The current status is never the only record of how it got there.

Payment status (independent machine, on `payments`):
`pending → paid | failed | expired`; `failed → pending` (retry); `paid → refunded`;
`expired → paid` (late callback).

## Alternatives considered

- **One combined status column** (today's shape). Rejected: cannot represent "paid but
  cancelled for stock reasons" or "unpaid but delivered", which is precisely the ambiguity
  that makes current revenue figures untrustworthy.
- **Full event sourcing.** Rejected as disproportionate for one shop; an append-only
  `order_status_history` gives the audit property without the operational burden. §14 lists
  overengineering as a risk.
- **Keeping `shipped` as the single post-preparation state.** Rejected: pickup has no
  shipping, and today pickup orders are pushed through a delivery-shaped vocabulary.

## Consequences

- The state machine lives in **one** Go package and is the only place transitions are
  decided. Route handlers and the frontend receive the permitted set from the API.
- Reservation expiry needs a scheduled job — the first real background job in the system
  (§8.3 of the inventory shows none exists today).
- The frontend's hardcoded status lists and the hidden client rule "delivered implies paid"
  (`api.jsx:620`) must be deleted; the API supplies both states.
- Existing sample/mock orders will not map cleanly. Pre-launch, so acceptable.

## Verification

- Unit tests: every permitted transition accepted; every pair not listed refused.
- Property test: no sequence of admin actions can move an order from `dispatched` to
  `cancelled`.
- Integration test: `expired → confirmed` on a late callback settles exactly once.
- API test: `PUT /orders/:id/status` with an illegal target returns 422 with the permitted
  set.

## Reopens if

the business adds a status (e.g. `partially_delivered`, `awaiting_payment_confirmation`) or
introduces multi-shipment split orders.

---

# ADR-016: Price authority for Admin 2

- **Status:** Accepted — **ratified by the product owner, 2026-11-05**, when asked whether
  pricing should stay above Admin 2; the answer was to keep it as written. No request/approval
  queue for price changes.

## Context

§4 gives Admin 2 the ability to "create, edit, publish, unpublish, archive, and maintain
product listings within granted scope" but does not say whether that includes price. §10
requires product/price changes to be audited. The current code has no distinction at all —
one admin role edits everything.

## Decision

**Admin 2 may set and change prices, with three constraints:**

1. Every price change requires a **reason** and writes to `audit_logs` with previous value,
   new value, actor, timestamp. No silent edits.
2. Price changes apply **prospectively only**. Order lines store a price snapshot at the
   time of purchase; a later edit never alters an existing order, invoice, or revenue
   figure. This is the doc's "pricing snapshot" in the Shopping context, made load-bearing.
3. Bulk or scheduled pricing (mass re-pricing, price lists, future-dated changes) is **not**
   in launch scope. A single-item edit is the only supported path.

## Alternatives considered

- **Admin 2 cannot touch price; Admin 1 approves each change.** Safer, but for a one-shop
  operation with a small team it creates a queue where a stock replenishment now needs two
  people. Rejected for launch; retained as the fallback if the first constraint proves too
  weak in practice.
- **No price audit.** Rejected outright — §10 names it explicitly.

## Consequences

`catalogue.products.update_price` is granted to Admin 2, Admin 1 and Super Admin. The
permission matrix's `‡` note is resolved. `order_lines.unit_price_cents` is immutable once
written.

## Verification

- Test: editing a product price does not change any prior order total or revenue aggregate.
- Test: a price change submitted without a reason is refused.
- Audit log query returns actor, before, after for every price edit.

## Reopens if

the business reports unexplained price drift, or asks for promotions to override base price
— in which case promotions need their own precedence rules, not a looser price permission.

---

# ADR-017: Review eligibility

- **Status:** Accepted — the rule is decided; implementation still cannot land before
  Phase 3 identity, because eligibility is proven against delivered orders that do not exist
  until customers do.

## Context

§4 requires a customer to "submit a review only for an eligible purchased product, subject
to review policy." Today `POST /api/reviews` (`index.js:410`) is **unauthenticated**, takes
any `product_id`, and has no purchase link — the `Review` model has no user reference at
all. So the rule is not merely unwritten, it is currently unrepresentable.

## Decision

A review requires all of:

1. **An authenticated Customer session.** Guest reviews are not accepted. This removes the
   public write endpoint and the free-text `name` field.
2. **A delivered order containing that product**, belonging to that customer. `delivered` is
   the trigger, not `paid` — a paid-but-cancelled order is not a real purchase.
3. **Within 90 days** of the order's delivery timestamp.
4. **One review per product per customer**, against the most recent qualifying order. A
   repeat purchase does not stack reviews.
5. Still subject to **moderation** before publication, as today.

Eligibility is computed by the backend from order records. The client may display an
"eligible" hint but the write is authorised server-side only.

## Alternatives considered

- **Eligible on `paid` rather than `delivered`.** Simpler, but lets people review orders
  that were later cancelled for stock reasons.
- **One review per order line** (so repeat buyers get multiple reviews). Rejected: inflates
  counts for a small catalogue and makes `rating_avg` drift with purchase frequency.
- **Auto-publish for verified purchases, no moderation.** Rejected — §4 keeps moderation, and
  the shop needs the ability to remove abusive content.

## Consequences

- Depends on customer identity, which does not exist today. **Review eligibility cannot ship
  before Phase 3 (Identity & RBAC).** Until then the public review endpoint should be
  disabled rather than left open.
- `reviews` gains `customer_id`, `order_id`, `status` (`pending|approved|rejected`),
  `approved_at`, `approved_by`.
- `rating_avg` / `rating_count` become **derived** from approved reviews, recomputed on
  approve/reject — and must be stored as a numeric type, not the string produced by
  `.toFixed(1)` today (`index.js:436`).

## Verification

- Test: a customer with no delivered order containing product X is refused.
- Test: a second review of the same product by the same customer is refused.
- Test: an unauthenticated POST to the review endpoint returns 401.
- Test: rejecting a previously approved review recomputes the average correctly.

## Reopens if

the business wants photo reviews, seller replies, or a public "verified purchase" badge with
different rules.

---

# ADR-018: Fulfilment model — zones, fees, pickup windows, and who may change them

- **Status:** Proposed — **the only decision still open.** The *shape* (zones, settings-backed
  fees, server-side store hours, Admin 1 configures / Admin 2 executes) is accepted and safe
  to build against. The *values* are commercial terms and are needed as input:
  1. Which areas does the shop actually deliver to, and are they grouped into named zones?
  2. Standard delivery fee, and the order value at or above which delivery becomes free.
  3. Same-day order cutoff time, and the days/hours that apply to pickup and to delivery.

## Context

Everything about fulfilment currently lives in the browser. `lib/fulfillment.jsx` holds the
delivery fee (KSh 150), the free-delivery threshold (KSh 600), opening hours per weekday, a
16:00 same-day cutoff, a "1–2 hours" pickup estimate, and the store location — all evaluated
against **the visitor's device clock**, and duplicated as static text in five places
(HomePage, AccountPage policy tab, Footer, LiveOverviewTab, and the module itself). There is
no delivery record in the database at all: `orders.shipping_address` is a free-text column.

## Decision

**Model and authority are defined now; the actual zone names and amounts are seeded as data
for the business to confirm.**

Structure:

- `delivery_zones` — `name`, `fee_cents`, `active`, `lead_time_hours`, `notes`. Zones are
  the priced unit. **Zone coverage (which places around Kakamega/Lurambi are served) is
  client input and is deliberately not invented here.**
- Free-delivery threshold and standard fee become **rows in `system_settings`**, not code
  constants, so Admin 1 can change them without a deploy. Current values preserved as
  defaults: standard fee **KSh 150**, free at subtotal **≥ KSh 600**.
- `store_hours` — per weekday, per fulfilment method, with an explicit
  `Africa/Nairobi` timezone. The device-clock evaluation is deleted; the server decides
  whether the store is open and says so.
- `deliveries` — one row per order needing delivery: zone, address, contact, fee charged,
  promised window, current state. `delivery_assignments` records who is taking it and when.
  `delivery_events` is the append-only status trail (§9: append-only history).
- Pickup orders get a `pickup_window` (date + time slot) and no address requirement.

Authority:

| Action | Who |
|---|---|
| Set zone coverage, fees, thresholds, store hours | **Admin 1** (`fulfilment.config.manage`) |
| Assign a delivery to a person, record dispatch/delivery | **Admin 2** (`fulfilment.assign`, `fulfilment.update`) |
| Change a customer's delivery address after confirmation | Admin 2, with reason, audited |

## Alternatives considered

- **Flat nationwide fee, no zones.** Matches today's behaviour and is simpler, but today's
  behaviour is a client-side constant nobody can change without a deploy. Zones are the
  minimum needed for the business to operate.
- **Per-distance pricing.** Rejected for launch: needs mapping/geo work and §6's "optional
  maps" is explicitly undecided.
- **Keeping hours in the frontend.** Rejected: it is the reason the cutoff message can be
  wrong for a visitor whose phone clock is off, and it cannot be changed without shipping
  a new bundle.

## Consequences

- All five duplicated copies of the hours text and both duplicated fee constants are
  removed; the API returns them.
- Currency arithmetic must follow ADR-019's money rules rather than client-side float
  addition.
- The delivery fee is computed **server-side at checkout** and snapshotted onto the order.
  A customer cannot lower their fee by editing a subtotal in devtools.

## Verification

- Test: fee and threshold come from settings; changing a setting changes the next checkout
  and not prior orders.
- Test: an order outside any active zone is refused at checkout.
- Test: outside opening hours the API reports closed using `Africa/Nairobi`, independent of
  client locale.

## Reopens if

the shop starts using riders with per-drop pricing, or expands beyond the county.

---

# ADR-019: Authentication and session model

- **Status:** Accepted — the remaining action is verification, not decision: confirm the
  cookie behaves correctly against the real deployment origin and TLS once hosting is chosen.

## Context

§18 leaves "secure cookie sessions versus access/refresh tokens" open. The current design is
a stateless 8-hour JWT (`auth.js:41`) with no role claim, accepted via `Authorization`
header **or `?token=` query string** (`middleware/auth.js:4`), and it is used for SSE
streaming and CSV export because those cannot set headers.

Two requirements decide this. **Assertion B11** (§19 step 2 matrix) requires that a
suspended or de-permissioned user fails on the *next request*, not the next login. **§10**
requires refresh-token rotation and revocation, and secure cookies where the design
supports them. A stateless JWT satisfies neither: an 8-hour token cannot be revoked without
a denylist, which is a session store wearing a disguise.

## Decision

**Server-side sessions with an `HttpOnly`, `Secure`, `SameSite=Lax` session cookie.**

- `sessions` table: opaque session ID (hash stored, not the raw ID), user, created/last
  seen/expires, IP and user-agent for audit, revoked flag.
- Sliding expiry with an absolute cap; idle timeout for admin roles shorter than for
  customers.
- **Revocation is immediate** — authorisation reads session + role permissions per request
  (with a short-lived in-process cache invalidated on change, satisfying B12).
- Password hashing: **Argon2id** with documented parameters (§10 names Argon2id or bcrypt;
  current bcrypt cost 10 is retained only as a migration path for existing rows).
- **CSRF protection** required on state-changing requests, because cookies are automatic.
- Daraja's callback is **not** a session-authenticated endpoint. It is verified by callback
  authenticity and an idempotency key (ADR-020, and §4 of the inventory).
- No credential is ever carried in a query string. This removes the `?token=` problem from
  the SSE stream and the CSV export: `EventSource` sends cookies same-origin, and export
  becomes a cookie-authenticated GET.
- Customer and admin sessions share the mechanism but not the trust level; admin sessions
  require an additional role check and are the shorter-lived class.

## Alternatives considered

- **Short-lived access token + rotating refresh token in an HttpOnly cookie.** Viable and
  common, and it is the right answer for a public API with third-party clients. Rejected
  for launch: this system has one first-party frontend, needs instant revocation, and the
  added complexity buys nothing here. Retained as the path if a mobile app ever needs to
  authenticate.
- **Stateless JWT with a revocation denylist.** Rejected — it is a session store plus a
  token, strictly more moving parts.
- **Keeping the current JWT.** Rejected: it fails B11 and B12 by construction.

## Consequences

- `CORS` must use an explicit origin allowlist with `credentials: true`; the
  `allowedOrigins = '*'` fallback (`server.js:21-34`) is removed.
- The three client-side token locations (`sessionStorage.adminToken`, the
  `localStorage.adminToken` read fallback, and legacy `localStorage.noja_token`) collapse to
  zero. The frontend stops managing tokens entirely, which also removes the fake-credential
  login path in `api.jsx:755`.
- Login rate limiting and lockout apply per account **and** per IP; today it is IP only.
- Sessions must survive a single-origin deploy on Vercel + a separate API host, so
  `SameSite` and domain settings need verifying against the real deployment, not assumed.

## Verification

- B11/B12 are directly testable: revoke a session, next request 401s.
- Test: a forged `?token=` request is refused.
- Test: CSRF token absent on a state-changing request → refused.
- Test: Argon2id parameters recorded and re-verifiable against a stored hash.

## Reopens if

a non-browser client is required, or the deployment cannot support credentialed
cross-origin cookies — in which case the refresh-token alternative becomes the decision.

---

# ADR-020: Refunds in launch scope, executed manually, recorded authoritatively

- **Status:** Accepted — **ratified by the product owner, 2026-11-05**, on the question of
  whether refunds are in launch scope with manual disbursement. Partial (per-line) refunds
  stand as written; they are not narrowed to full-order refunds.

## Context

§18 lists refunds as undecided, and §9 lists a `refunds` table "(if refunds are
introduced)". The current system has no refund concept whatsoever: no table, no endpoint, no
UI. Payment status can only be flipped by hand in a dropdown with no reason and no record.

## Decision

**Refunds are in launch scope, but the money moves manually and the system is the
authoritative record.**

- A refund is **requested** against an order in `returned` or `cancelled` status, with a
  mandatory reason and the amount, defaulting to the paid amount and permitting a lesser
  figure with justification.
- **Approval** requires `payments.refund` — Admin 1 or Super Admin only. An Admin 2 may open
  a request but may never approve one.
- **Execution happens outside the application** — via the Daraja portal or the till, by a
  human with the store's M-Pesa administrative access. The system records that it happened:
  operator, timestamp, Daraja transaction reference, amount.
- **Automated B2C reversal is explicitly out of launch scope.** It is a second live-money
  integration with its own failure modes, and adding it now would double the payment surface
  that ADR-015 and the inventory's §4 findings are still being secured against.
- Refunding does **not** automatically restock. Goods arrive in `returned` and re-enter
  inventory only via an audited `inventory.stock.adjust` after inspection. This keeps the
  money and stock machines independent, which is the entire point of ADR-015.
- `payments.status` moves `paid → refunded`; the order moves to `refunded` only when the
  recorded refund amount equals or exceeds the paid amount.

## Alternatives considered

- **Defer refunds entirely; handle them in a notebook.** Rejected: the shop would have no
  record of money it has paid back, and revenue reporting would stay permanently wrong.
- **Automated Daraja B2C reversal.** Rejected for launch as above; revisit once the
  callback verification and idempotency work in ADR-015 and §4 of the inventory is proven in
  production.
- **Auto-restock on refund.** Rejected: assumes returned goods are resellable, which for
  electricals is not a safe assumption.

## Consequences

- `refunds` table: order, payment reference, requested amount, approved amount, reason,
  requester, approver, execution reference, executed_at, status
  (`requested|approved|executed|rejected`).
- `payments.refund` becomes a real permission for Admin 1 and Super Admin; the matrix's
  "blocked for every role" note is resolved.
- Revenue reporting must subtract executed refunds. The current rule
  (`payment_status='paid' OR status='delivered'`) becomes indefensible and must be replaced
  with an explicit revenue definition — recorded as an open item for §19 step 3.
- The system cannot *prevent* an operator from sending money that isn't recorded. The
  control is that an unrecorded refund is a policy violation, detectable by reconciling the
  till statement against `refunds`.

## Verification

- Test: Admin 2 cannot approve a refund (B3/B6 class).
- Test: a refund request against a `delivered` order with no `returned` transition is refused.
- Test: partial refund leaves the order in `returned`, not `refunded`.
- Reconciliation report: refunds recorded vs till-statement outflows, for a period.

## Reopens if

refund volume makes manual execution the bottleneck, at which point Daraja B2C becomes the
next payment integration.

---

# ADR-021: Money representation — integer cents, whole-shilling prices, round once

- **Status:** Accepted
- **Date:** 2026-11-05
- **Lifted from:** `docs/phase-1-domain-model-erd.md` §2, where it was recorded as a candidate
  for this register.

## Context

§9's data rules forbid floating-point money. Two constraints collide here: Daraja STK Push
requires a **whole-number shilling** amount, while product prices and percentage discounts
naturally produce fractional shillings. The current code resolves the collision by rounding at
the moment of payment (`Math.ceil(amount)` in `mpesa.js:65`) on a total the client supplied —
so the rounding is invisible, unrepeated and unrecorded, and the amount charged can differ
from the amount the order says is owed.

## Decision

1. Every monetary value is a `BIGINT` count of **cents**. No float, no numeric-in-a-string, no
   "KES 1,200" as a value.
2. **Product prices are whole shillings**, enforced in the database:
   `CHECK (price_cents % 100 = 0)`.
3. A percentage discount is computed in cents and **rounded half-up to the nearest whole
   shilling exactly once, at quote creation**, then snapshotted onto the order line. Nothing
   downstream re-derives it.
4. `totals_reconcile` (`total = subtotal + delivery_fee − discount`) and
   `line_total_is_exact` (`line_total = unit_price × quantity`) are database CHECKs, so a
   response whose arithmetic does not close cannot be persisted, let alone returned.

Consequence of 2 + 3: `amount_cents / 100` sent to Daraja is **identical** to the order total,
not approximately equal to it. That is what lets `paid_requires_exact_amount` exist at all —
an exact-match settlement rule is only fair when the system never generated a fraction.

## Alternatives considered

- **Allow sub-shilling prices and round at payment time.** Rejected: it reintroduces the
  current defect, and forces the reconciliation job to treat "off by a few cents" as a
  routine state rather than a bug.
- **Store money as `NUMERIC(12,2)`.** Rejected against §9's wording, and it invites the
  arithmetic to leak into whatever language reads it.
- **Round discounts per line instead of per order.** Rejected: per-line rounding makes the
  order total the sum of independently rounded parts, which drifts from the whole-order
  figure the customer was quoted.

## Consequences

- The shop cannot list a KSh 99.99 price. If sub-shilling pricing is ever genuinely required,
  this ADR is the one to reopen, and `paid_requires_exact_amount` must be relaxed in the same
  change — not left to disagree with it.
- Daraja integration gets simpler: no `ceil`, no amount tolerance, no "close enough" branch.
- Every API field carrying money is an `int64` named `*_cents`, and currency appears only on
  `system_settings` and `payment_transactions` (§9 selective-JSONB discipline aside, a
  single-currency shop does not repeat it per row).

## Verification

- Unit: a 12.5%-off KSh 1,499 line produces one recorded rounded figure, and re-quoting it
  returns the same figure.
- Property test: for any set of lines, `sum(line_total_cents) == subtotal_cents` and the
  quoted `total_cents` equals the amount handed to Daraja.
- Database: an attempt to insert a price with non-zero cents is refused; an order whose totals
  do not close is refused.

## Reopens if

the business adopts sub-shilling pricing (unit economics, fuel-indexed goods) or adds a second
currency.

---

# ADR-022: Revenue recognition — settlement minus executed refunds

- **Status:** Accepted — **decided by the product owner, 2026-11-05**, when asked which
  definition the analytics should implement.
- **Date:** 2026-11-05

## Context

The current rule counts an order as revenue when `payment_status='paid' OR status='delivered'`
(`index.js:530,557`). The `OR` books goods that were delivered but never paid for, and the two
branches measure different things, so the same day can be reported as revenue twice depending
on which filter an endpoint applies. ADR-020 additionally puts refunds in launch scope, and a
definition that ignores them overstates the till.

## Decision

**Revenue = Σ `payments.amount_paid_cents` where `payments.status = 'paid'`, minus Σ
`refunds.amount_cents` where `refunds.status = 'executed'`, each attributed to the period in
which that settlement or that execution occurred — not the period the order was created.**

Two corollaries that make the definition implementable rather than aspirational:

- A `pending` order is **not** revenue, however far along its fulfilment has got. Order count
  and revenue are separate metrics and must never be conflated in one card.
- A refund subtracts in the period it was **executed**, not the period of the original sale.
  Restating a closed historical period is not something a shop owner can verify against their
  own till statement; the current period absorbs the reversal.
- Manual overrides and reconciled settlements are tagged at source (`reconciliation_runs`), so
  "callback-confirmed revenue" and "revenue established by hand" can be reported apart. The
  headline figure is the same; the audit view shows its composition.

## Alternatives considered

- **Recognise at delivery.** Rejected — it decouples the revenue figure from the money in the
  till, which is the comparison the owner actually makes.
- **Keep the existing dual rule for continuity with numbers already shown.** Rejected: it is
  the defect the Phase 0 inventory flagged, and pre-launch means no published history has to
  be reconciled with it.
- **Match refunds back to the original sale period.** Rejected for the till-statement reason
  above; revisit only if accounting advice says otherwise.

## Consequences

- `daily_snapshots` and `sales_summaries` key on `payments.settled_at` and
  `refunds.executed_at`, so they cannot be derived from `orders.created_at` at all. Any
  dashboard that groups by order date must be labelled "orders by date", not revenue.
- Every figure the current analytics panel computed is wrong under this definition and is not
  migrated — the Phase 7 work is a rebuild, not a port.
- `refunded` orders that were never `paid` (cancelled before settlement) produce no revenue and
  no refund line, which is correct and will look like a gap to anyone expecting symmetry.

## Verification

- Test: a delivered-but-unpaid order contributes 0 revenue and 1 order.
- Test: a refund executed in period B reduces period B, leaves period A untouched.
- Test: `revenue(orders) − revenue(refunds)` equals the till statement total for a period,
  given the provider record as ground truth.
- Test: an override-settled payment appears in the audit view distinctly from callback revenue.

## Reopens if

the business needs accrual-basis reporting, an auditor requires refunds matched to their
original period, or a second currency makes a single `amount_paid_cents` column ambiguous.

---

# ADR-023: One stock location

- **Status:** Accepted — **decided by the product owner, 2026-11-05.**
- **Date:** 2026-11-05

## Context

§18 asked whether stock is tracked per location. The target schema states
`inventory(product_id UNIQUE, on_hand, reserved)`, which asserts a single location: one row
per product, one availability number, and a conditional update that reserves against it. That
assertion is load-bearing for T-01, the reservation path that prevents overselling, and it was
recorded as "confirm before Phase 4".

## Decision

**One stock location.** `inventory.product_id UNIQUE` stays. Availability shown on the
storefront is `on_hand − reserved` for that one location, and the low/critical thresholds in
`system_settings` apply to it.

## Alternatives considered

- **Multiple locations now.** Rejected on cost, not on principle: the key becomes
  `(product_id, location_id)`, `stock_movements` and `stock_reservations` gain a location, the
  reservation update must choose a location (introducing allocation rules), transfers become
  first-class movements, and fulfilment must resolve which location serves which order. That is
  a second inventory system, and the operation is one shop with one till and one pickup point.
- **One location today, no record of the change cost.** Rejected — an unrecorded assumption is
  how Phase 4 turns into a rewrite.

## Consequences

- Pickup and delivery both draw on the same availability pool. If delivery ever ships from
  different premises, this ADR is wrong and the schema changes underneath it.
- Multi-location stock is **not** expressible without a migration that changes a unique key on
  the hottest table in the system. Doing it later is possible and priced: new
  `fulfilment.locations`, key widening on three tables, and a backfill assigning every existing
  row to the original location.
- `orders` therefore needs no location column, and the ERD's `available GENERATED ALWAYS AS
  (on_hand - reserved) STORED` stays a plain per-product generated column.

## Verification

- Test: concurrent reservations across two carts against one product cannot take `available`
  below zero (this is T-01's real assertion and holds precisely because there is one row).
- Test: a stock adjustment writes exactly one movement with the correct `on_hand_after`.

## Reopens if

a second physical premises, a consignment partner, or per-branch reporting is introduced — or
if delivery is ever fulfilled from somewhere other than the shop floor.

---

# Deferred by these decisions

Two §18 items are **not** resolved here, deliberately:

- **Product variants and SKU rules.** §9 marks `product_variants` as "(if variants are
  required)". A `sku` column with a uniqueness constraint is added now, because §4 requires
  SKU uniqueness and the current `Product` model has no SKU field at all. Variant *handling*
  (size/capacity/plug-type permutations with per-variant stock) is deferred — it changes the
  inventory model substantially and nothing in the current system needs it yet. Revisit when
  a real listing requires it.
- **Data retention, privacy notice, consent and deletion processes** (§18, Kenyan
  requirements). This is a legal/commercial input, not an engineering decision. Flagged as
  unresolved and blocking for launch, because the system stores names, phones, emails and
  addresses.
