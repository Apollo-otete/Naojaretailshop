# Phase 0 — Discovery & Safeguards: Current-Behaviour Inventory

**Project:** Naoja Retail Shop · **Status:** Working baseline for review
**Scope:** Existing Express/React implementation, as-is, before any Go rewrite
**Source of authority:** `Naoja_Retail_Shop_Architecture_and_Reform_Decisions.pdf` §12 step 2, §13 Phase 0

This document records what the system *actually does today*, so that business rules are
migrated deliberately rather than discarded during the rewrite. It is a description of
current behaviour, not a target design.

---

## 0. How to read this document

Findings are marked by verification status:

- **[V]** — read directly in source and confirmed line-by-line.
- **[R]** — reported by an automated sweep of `frontend/src/` and spot-checked, but not
  every claim independently re-verified. Re-confirm before relying on one for a decision.

Severity markers: **CRITICAL** (financial/data-integrity exposure), **HIGH**, **MEDIUM**.

---

## 1. Runtime shape

| Layer | Today | Notes |
|---|---|---|
| Backend | Node + **Express 5.2.1** (`backend/package.json`) | Not Express 4 — matters when porting examples |
| Frontend | React 18 + Vite 7 + Tailwind, plain JSX (no TypeScript) | `frontend/src/` ≈ 9,100 LOC |
| Transactional DB | PostgreSQL (`pg` pool) | Orders, subscribers, messages, admins, analytics |
| Catalogue DB | **MongoDB via Mongoose** — mandatory, no fallback | Products, categories, reviews |
| Payments | Safaricom Daraja STK Push | Sandbox + "production" creds (see §7) |
| Realtime | Hand-rolled in-memory SSE bus | Single-process only |
| Email | Nodemailer SMTP | **Currently disabled** (see §8) |
| Background jobs | **None exist** | No scheduler dependency at all |
| Process model | One Express app, `node server.js` | Graceful SIGTERM/SIGINT shutdown |

Startup sequence (`backend/server.js:86-103`): connect databases → initialise Postgres
tables → listen. **The server starts successfully even if every database is offline**
(`db.js:40-43` catches and does not exit). This is the root enabler of the masking
problem described in §3.

---

## 2. HTTP endpoint inventory

### 2.1 Catalogue — MongoDB-backed (`backend/src/routes/index.js`)

| Method | Path | Auth | Behaviour / rules |
|---|---|---|---|
| GET | `/api/products` | none | Filters by `category` (slug→`category_id` lookup), `search` (`$regex` over name/description/subcategory, case-insensitive). Pagination via `page`/`limit` (default limit **12**) returns `{products, pagination}`; without pagination returns a bare array. Sets `X-Total-Count`. Sorts by `id` ascending. **[V]** |
| GET | `/api/products/:slugOrId` | none | Matches slug, or slug **or** numeric `id`. **Increments `views` on every read** (`findOneAndUpdate $inc`). 404 if absent. **[V]** |
| POST | `/api/products` | admin | `id` = max existing id + 1. Slug = `req.body.slug` or generated from name. Invalidates analytics cache. **[V]** |
| PUT | `/api/products/:id` | admin | If `stock_quantity` present, **derives `status`**: ≤0 `out_of_stock`, ≤5 `low_stock`, else `in_stock`. **[V]** |
| DELETE | `/api/products/:id` | admin | Hard delete. **[V]** |
| GET | `/api/categories` | none | All categories, sorted by `id`. `product_count` is a stored field, not computed. **[V]** |
| POST/PUT/DELETE | `/api/categories[/:id]` | admin | Same id+1 and slug rules as products. **[V]** |
| GET | `/api/reviews` | none | Public view returns **only `is_approved: true`**; `?admin=true` returns all. **[V]** |
| POST | `/api/reviews` | **none** | No auth, **no purchase-eligibility check**. Defaults to unapproved. **[V]** |
| PUT | `/api/reviews/:id/approve` | admin | Recomputes product `rating_avg` from approved reviews via `.toFixed(1)` → **stores a string in a Number field**. **[V]** |
| PUT | `/api/reviews/:id/reject` | admin | Sets `is_approved: false`. **[V]** |
| POST | `/api/upload` | admin | Multer disk storage → `backend/uploads/`, 10 MB cap, `image/*` only. Returns URL built from `req.get('host')`. **[V]** |

### 2.2 Commerce — PostgreSQL-backed

| Method | Path | Auth | Behaviour / rules | **Severity** |
|---|---|---|---|---|
| GET | `/api/orders` | admin | Paginated (default limit 20), newest first. | |
| POST | `/api/orders` | **none** | Accepts `total_amount`, `payment_status`, `mpesa_transaction_id`, `mpesa_checkout_request_id` **directly from the request body** and inserts them. Generates `order_ref`. Decrements Mongo stock. Fires emails + SSE. | **CRITICAL** |
| GET | `/api/orders/track` | none | Matches `order_ref` (case-insensitive) + `customer_phone LIKE '%<last 9 digits>'`. | |
| PUT | `/api/orders/:id` | admin | Sets `status` and/or `payment_status` to **any value, no transition validation**. If `status==='cancelled'`, restores Mongo stock. | **HIGH** |
| GET | `/api/newsletter` | **none** | Returns entire subscriber table. | **CRITICAL** |
| POST | `/api/newsletter` | none | Idempotent on existing email (200, not duplicate). | |
| GET | `/api/contacts` | **none** | Returns every contact message incl. name, phone, email. | **CRITICAL** |
| POST | `/api/contacts` | none | Inserts message. | |
| PUT | `/api/contacts/:id/read` | admin | Sets `is_read = true`. | |
| GET | `/api/analytics` | admin | Legacy dashboard aggregate (see §5.4 for the revenue rule). | |
| GET | `/api/health` | none | `{status, timestamp, uptime}`. **Reports healthy even with zero databases connected.** | |

### 2.3 Payments (`backend/src/routes/mpesa.js`)

| Method | Path | Auth | Notes | **Severity** |
|---|---|---|---|---|
| POST | `/api/mpesa/stkpush` | **none** | Rate-limited 6/min/IP. `Amount: Math.ceil(amount)` taken from the **client**, never validated against the order total. Phone normalised to `254XXXXXXXXX`. Stores `CheckoutRequestID` on the order. | **CRITICAL** |
| POST | `/api/mpesa/callback` | **none** | Safaricom-facing. See §4 — unauthenticated, amount unchecked, non-idempotent, phone-fallback misattribution. | **CRITICAL** |
| POST | `/api/mpesa/query` | **none** | Calls Daraja `stkpushquery`, then **writes `payment_status='paid'` to the DB** based on the caller-supplied `checkoutRequestId`. Code `1037` treated as in-progress. | **CRITICAL** |
| GET | `/api/mpesa/order-status/:orderRef` | **none** | Returns status, payment status, transaction ID, amount. Refs are sequential → **enumerable**. | **HIGH** |

### 2.4 Auth (`backend/src/routes/auth.js`)

| Method | Path | Auth | Notes |
|---|---|---|---|
| POST | `/api/auth/login` | none | Rate-limited 15/15min/IP. **Two independent paths** (see §6.2). JWT 8h expiry, payload `{id, email, name}` — **no role or permission claim**. |
| PUT | `/api/auth/change-password` | admin | bcrypt cost 10. Requires `newPassword.length >= 6`. Non-functional in practice — see §8.1. |

### 2.5 Analytics (`backend/src/routes/analytics.js`)

All admin-guarded except as noted. Base `/api/analytics`.

`GET /events/stream` (SSE) · `GET /live` · `GET /summary` · `GET /category-performance` ·
`GET /inventory/health` · `GET /forecast/revenue` · `GET /forecast/insights` ·
`GET /payment-health` · `GET /fulfillment` · `GET /alerts` ·
`POST /alerts/:id/acknowledge` · `GET /targets/progress` · `POST /targets` · `GET /export`

`/events/stream` and `/export` authenticate via `?token=` because `EventSource` and
`window.open` cannot set headers. `middleware/auth.js:4` also accepts `req.query.token`,
so these work — but the token is then captured in nginx/proxy access logs and browser
history. **[V]**

---

## 3. The masking problem — highest-priority planning finding

Two independent client-side fallback layers make a broken backend indistinguishable from
a healthy one.

**3.1 `frontend/src/lib/api.jsx` — a complete fake backend in `localStorage`** [V]

`request()` wraps the fetch in `try { … } catch { handleLocalFallback() }`
(`api.jsx:838-860`). The `catch` therefore intercepts **all three** of:

- network failure / backend down,
- `response.json()` parse failure (an HTML error page, or a Vite proxy miss),
- **any HTTP 4xx/5xx**, because `!response.ok` throws at `:843`.

`handleLocalFallback()` (`:374-815`) implements GET/POST/PUT/DELETE for products,
categories, orders (including tracking and stock decrement), reviews (with rating
recompute), newsletter, contacts, upload, analytics — **and authentication**. It seeds
14 categories, 12 products, 3 orders, 3 reviews, 3 subscribers and 2 messages
(`:7-335`), and adds a 300 ms artificial latency so the UI feels normal.

Consequences:

- An expired admin token (401) renders a fully populated dashboard from mock data.
- Order placement "succeeds" and writes to `localStorage` while nothing reaches the server.
- Analytics KPIs are computed from fabricated orders.
- **`api.jsx:755` hardcodes the live owner credentials** `naojaventures@gmail.com` /
  `naoja@1540` in the shipped bundle, and returns a fake token on match. `:764-766` makes
  `change-password` return a success message that never touched a server.
- Any `naoja_db` content in a browser is **test residue, not data**. It must not be
  migrated or reconciled.

**3.2 `frontend/src/lib/analyticsApi.js` — in-memory mocks** [R]

Separate mock set per endpoint (live events, random-walk sales trend, inventory, category
performance, insights, payment health, fulfilment funnel, alerts, target progress).
Falls back whenever the fetch throws **or `!res.ok`**, silently. Same masking class of
problem, different mechanism, and it bypasses the `request()` wrapper entirely.

**Planning implication:** before any cutover decision can be made, these must be disabled
or made loudly visible. Until then, no one — including the owner — can tell whether the
production site is functioning. This also means **the frontend cannot be used to validate
the Go backend**; a misconfigured Go service will look identical to a working one.

---

## 4. Payment integrity — current vs. required

The reform document states the rule: *"a browser redirect or frontend success message is
not proof of payment. The authoritative payment state must be established by the backend
from the Daraja callback/reconciliation process."* Current behaviour violates it at every
step. **[V]**

| # | Defect | Location |
|---|---|---|
| 4.1 | Callback endpoint is publicly POSTable. No signature, allowlist, shared secret or IP verification. A forged `{Body:{stkCallback:{ResultCode:0}}}` marks an order paid. | `mpesa.js:116` |
| 4.2 | **`amountPaid` is read from callback metadata, logged and broadcast — but never compared to `orders.total_amount`.** KES 1 settles a KES 10,000 order. | `mpesa.js:138,183-188` |
| 4.3 | Success-path `UPDATE` has **no `AND payment_status='pending'` guard**, while the failure path (`:197`) and query path (`:251`) both do. Replayed callbacks re-settle and re-email. No idempotency key anywhere. | `mpesa.js:146-154` |
| 4.4 | Phone-suffix fallback updates **any** pending order matching the last 9 digits, most recent first — a new payment can settle an unrelated older order. | `mpesa.js:163-180` |
| 4.5 | `POST /orders` trusts client-supplied `payment_status`, so an order can be created already paid. | `index.js:279-312` |
| 4.6 | STK amount comes from the client, unvalidated against the order. | `mpesa.js:46,65` |
| 4.7 | `/api/mpesa/query` is unauthenticated and writes `paid` to the DB from a caller-supplied `checkoutRequestId`. | `mpesa.js:220-262` |
| 4.8 | **Callback URL mismatch:** `MPESA_CALLBACK_URL` is `…/api/payment/callback`; the route is mounted at `/api/mpesa/callback`. Every real Safaricom callback 404s. Settlement only ever happens through frontend polling of `/query`. | `server.js:73` vs env |
| 4.9 | Client decides payment success: polls status, flips UI to "Paid", clears the cart. The manual path completes an order from a **self-typed M-Pesa code** with no verification. | `CheckoutPage.jsx:101-144,181-194,198` |
| 4.10 | Admin can flip `payment_status` to `paid` from a dropdown — currently the only "reconciliation" for manual till payments, with no reason, actor or audit record. | `AdminDashboard.jsx:455-467,1494-1503` [R] |

**Rules to carry into the Go design:** payment state derives only from a verified callback
or an authoritative `stkpushquery`; the paid amount must equal the order total; each
`CheckoutRequestID` settles at most once; the STK amount is read from the server-side
order, never from the client; manual overrides require an explicit reason and an audit
entry.

---

## 5. Stock, orders and totals

### 5.1 Overselling is guaranteed [V]

Stock decrement is a non-atomic `$inc` loop executed **after** the order row is inserted
(`index.js:315-322`). There is no availability check, no row lock, no transaction, and no
reservation concept. Two concurrent checkouts both succeed against one unit.

### 5.2 Cross-database write with no transaction boundary [V]

Stock lives in **MongoDB**; orders live in **PostgreSQL**. The two cannot be committed
together. If the process dies between the insert and the decrement, the order exists with
stock unconsumed. This is the strongest concrete argument for ADR-001 in the whole audit.

### 5.3 Cancel inflates inventory [V]

Cancel restores stock (`index.js:374-388`) with no guard against double-processing.
Cancelling twice adds stock twice.

### 5.4 Order reference generation races [V]

`order_ref = NJ-<year>-(count(*)+1)` (`index.js:289-291`). Concurrent orders compute the
same value → `UNIQUE` violation and a failed checkout. Deleting any order causes future
collisions. Also exposed through an unauthenticated sequential-id endpoint (§2.3).

### 5.5 Revenue definition is ambiguous [V]

Total sales count an order as revenue when `payment_status = 'paid' **OR**
status = 'delivered'` (`index.js:530,557`; mirrored client-side at `api.jsx:770-772`,
`AdminDashboard.jsx:294-296`). A delivered-but-unpaid order is booked as revenue. The
client-side fallback also encodes a hidden rule that `status:'delivered'` implies paid
(`api.jsx:620`). **This must be pinned down before the analytics engine is rewritten.**

### 5.6 No order state machine [V]

`PUT /orders/:id` accepts any string for `status`. The vocabulary in use —
`pending / confirmed / shipped / delivered / cancelled` and `pending / paid / failed` —
appears only in a SQL comment (`naoja_schema.sql:59,61`), a schema enum, and frontend
`<select>` lists. No transitions are enforced anywhere. Reform doc §18 already lists this
as an open item; this confirms it is genuinely undefined.

### 5.7 Money types

Prices are `DECIMAL(10,2)` in Postgres and `Number` in Mongoose. STK uses `Math.ceil`.
Client formatting is `maximumFractionDigits: 0` (`emailService.js:35`) and
`KSh ${x.toLocaleString()}` across the UI, while analytics uses `Intl` with `KES` — two
currency label conventions. **KES has no practical sub-unit, so the doc's "integer minor
units in cents" rule needs a decision about whether cents exist at all in this business.**

---

## 6. Identity, roles and permissions

### 6.1 What exists

One admin concept. `middleware/auth.js` verifies a JWT and sets `req.admin`; **there is no
role check and no permission check anywhere.** Any valid token grants every admin route.
Customer identity **does not exist**: no registration, no login, no `users` table in the
live schema, no ownership of orders. Customers are anonymous, identified by name + phone
entered at checkout. **[V]**

### 6.2 Login has two paths, one of which is a backdoor [V]

1. `auth.js:28` compares the submitted password to `ADMIN_PASSWORD` as a **plaintext
   string** (so the env value must be plaintext) and issues a token on match.
2. `auth.js:59-79` falls back to bcrypt against the `admins` table.

Because path 1 always runs first, the env credential is a permanent alternative password
that cannot be rotated through the application. `auth.js:122` extends the same bypass to
`change-password`.

### 6.3 Secret handling [V]

`middleware/auth.js:12` and `auth.js:21` fall back to `'fallback_dev_secret_key_123'`
unless `NODE_ENV === 'production'`. **`analytics.js:23` (SSE) omits that production
guard entirely** — it accepts the hardcoded dev secret unconditionally. If `JWT_SECRET`
is ever unset, or a token is presented to the SSE route, a publicly known signing key is
accepted.

### 6.4 Gap to the target model

The five-actor model (Guest, Customer, Admin 2, Admin 1, Super Admin) with granular
permissions (`products.create`, `orders.process`, `audit_logs.view`) has **no foundation in
the current code** — no users, no roles, no permissions, no sessions, no invitations, no
audit log. Phase 3 is a full build, not an extension.

For permission assignment, today's admin surface is 13 tabs, not 8 [R]:

| Tab | Capability | Suggested tier |
|---|---|---|
| Live Command | Read ops feed; mute chime | Admin 2 |
| Sales & Revenue | Read KPIs/trends; range slicer | Admin 1 |
| Predictive Forecast | **Set revenue/order targets** | Admin 1 (or Super) |
| Stock Intelligence | Read velocity; **export PO CSV** | Admin 2 |
| M-Pesa Diagnostics | Read gateway health | Admin 1 / Super |
| Logistics Funnel | Read | Admin 2 |
| Alerts Centre | **Acknowledge alerts** | Admin 2 |
| Products | **Full CRUD + image upload** | Admin 2 (price edits → Admin 1?) |
| Categories | **CRUD**; delete orphans products | Admin 2 |
| Orders | **Change status; override payment_status** | Admin 2; payment override needs Admin 1 + audit |
| Reviews | **Approve / reject** | Admin 2 |
| Inquiries | Mark read/unread | Admin 2 |
| Subscribers | Read | Admin 1 |
| Session | Change password, sign out | Super Admin |

---

## 7. Data structures

### 7.1 Authoritative Postgres schema = `config/initPg.js`, run at every boot [V]

Tables: `orders`, `subscribers`, `messages`, `admins`, `analytics_daily_snapshot`,
`analytics_alerts`, `analytics_targets`, `analytics_thresholds`,
`product_sales_velocity`. Indexes on snapshot date and alerts created.

`orders` columns: `id, order_ref, customer_name, customer_phone, customer_email,
total_amount, status, payment_method, payment_status, mpesa_till_number,
mpesa_transaction_id, mpesa_checkout_request_id, shipping_address, notes, items (JSONB),
created_at`.

**Note there is no `updated_at` on `orders` here**, and no foreign keys, no `order_items`,
no `deliveries`, no `stock_movements`, no `audit_logs`, no `users`.

### 7.2 `migrations/naoja_schema.sql` is not a migration and does not describe this app [V]

It defines `users` (with `role`), `categories`, `products`, `orders`, `order_items`,
`reviews`, `subscribers`, **`contact_messages`** (the code uses `messages`), `cart`,
`cart_items`, plus `vendor_id`, `parent_id`, a seeded `admin@naojaventures.com` with a
bcrypt hash commented `admin123`, and 14 categories under a heading that says "23
categories". **None of it matches `initPg.js`.**

It is only touched by `scripts/full-sync.js`, which regenerates it with `pg_dump -s` every
30 seconds (§8.4). It is a generated artifact of some other/older database, hand-edited
afterwards. **Do not use it as the basis for the new schema, and do not run it.**

### 7.3 MongoDB collections (Mongoose) [V]

- `Product`: `id` (Number, unique — *not* the Mongo `_id`), `name`, `slug`, `description`,
  `price` (Number), `stock_quantity`, `category_id` (Number), `subcategory`, `images[]`,
  `status` enum, `is_featured`, `views`, `rating_avg`, `rating_count`, timestamps.
  **No `sku` field, no `brand`, no `specifications`, no publication state.**
- `Category`: `id`, `name`, `slug`, `icon` (emoji), `product_count` (stored, not computed).
- `Review`: `id`, `product_id`, `name` (free text), `rating` 1–5, `comment`,
  `is_approved` (default false), `created_at`. **No user reference → purchase-eligibility
  enforcement is impossible today.**

### 7.4 Migration implications

Manual id sequences (`id = max+1`) exist because the app uses application-level integer
ids across two databases. The rewrite should expect: duplicate/orphaned catalogue rows,
`rating_avg` stored as strings, `items` as opaque JSONB with no normalised order lines,
`product_count` out of sync with reality, and no referential integrity anywhere.

---

## 8. Broken or dead functionality

### 8.1 Admin password changes are silently reverted [V]

`initPg.js:148-166` runs on **every boot**: it hashes `ADMIN_PASSWORD` (default
`naoja@1540`) and **overwrites the `admins` row's password**. So a password changed via
`/api/auth/change-password` is wiped on the next restart, and §6.2's env bypass means the
old credential still works regardless. Combined effect: **password rotation is
non-functional.**

### 8.2 Email is disabled [V]

`emailService.js:12` returns `null` when `EMAIL_USER`/`EMAIL_PASS` are unset — they are
unset. Every send site uses `.catch(e => console.warn(...))`, so failures are invisible.
No order confirmations and no admin alerts are being delivered.

### 8.3 No cron / background jobs [V]

`CRONJOBS_ENABLED` and `CRON_SECRET` are read by **nothing** in the codebase, and there is
no scheduler dependency. The doc's "scheduled tasks and retriable work", "scheduled
reminders", retry-on-failure email, and nightly analytics snapshot are all greenfield.
`analytics_daily_snapshot` is only populated when something calls it.

### 8.4 `full-sync.js` is broken and hazardous [V]

Requires `chokidar`, which is **not in `package.json` and not installed** → crashes on
startup. If it did run, it would: overwrite the tracked `naoja_schema.sql` from live DB
state every 30 s; apply `psql -f` on any edit to that file (a feedback loop with the
export); and interpolate `PGPASSWORD` into a shell string, exposing the DB password in
process argv and inviting shell injection. **Recommend deleting it.**

### 8.5 Orphaned frontend code [R]

`AdminLogin.jsx` is imported by nothing and has no route, yet `handleLogout` navigates to
`/admin/login` (`AdminDashboard.jsx:547`) — a path that renders nothing. `CategoryCard.jsx`
likewise unused. **`/admin` has no client-side guard at all.**

### 8.6 CORS wildcard with credentials [V]

`server.js:21-34`: if `FRONTEND_URL` is unset, `allowedOrigins` becomes the string `'*'`
and the origin callback admits everything, with `credentials: true`.

---

## 9. Environment variable audit [V]

Read by the code: `PORT`, `NODE_ENV`, `FRONTEND_URL`, `JWT_SECRET`, `PG_HOST`, `PG_PORT`,
`PG_USER`, `PG_PASSWORD`, `PG_DATABASE`, `MONGODB_URI`, `MPESA_CONSUMER_KEY`,
`MPESA_CONSUMER_SECRET`, `MPESA_SHORTCODE`, `MPESA_PASSKEY`, `MPESA_BASE_URL`,
`MPESA_CALLBACK_URL`, `EMAIL_USER`, `EMAIL_PASS`, `EMAIL_SERVICE`, `EMAIL_FROM`,
`ADMIN_EMAIL`, `ADMIN_PASSWORD`.

**Read by nothing:** `SUPABASE_*`, `CLOUDINARY_*`, `NEON_*`, `PUSHER_*`, `RESEND_API_KEY`,
`CRONJOBS_ENABLED`, `CRON_SECRET`, `ENVIRONMENT`, `ENVIRONMENT_PROD`, and every unprefixed
Daraja name (`CONSUMER_KEY`, `SHORTCODE`, `PASSKEY`, `CALLBACK_URL`, `*_PROD`). No SDK
dependency exists for any of those providers.

**Set but wrong:** `MONGODB_URI` absent (catalogue 500s without it); `EMAIL_*` absent
(email off); `ADMIN_EMAIL`/`ADMIN_PASSWORD` absent (code uses committed defaults);
`MPESA_CALLBACK_URL` points at a non-existent route; the "production" Daraja consumer
key/secret are **copies of the sandbox values** with a production shortcode, so a
production swap will fail OAuth; `MPESA_PASSKEY` is Safaricom's public sandbox default.

---

## 10. Business rules to preserve

These are the rules that exist today and must be re-implemented deliberately, not
rediscovered later.

**Catalogue**
- Low-stock threshold **5** appears in three places with two mechanisms: hardcoded
  `≤5` in `index.js:133`, hardcoded `$lte: 5` in `index.js:536`, and configurable
  `low_stock_limit: 5` in `analytics_thresholds`. One source of truth required.
- Out-of-stock at ≤0; `status` is derived from quantity, never set independently.
- Slug rule: `name.toLowerCase().replace(/[^a-z0-9]+/g, '-')`, applied to products and
  categories; explicit slug wins.
- `views` increments on every product detail read.
- Pagination default 12 (storefront), 20 (admin orders).

**Thresholds** (`initPg.js:131-142`, JSONB, editable) [V]
`low_stock_limit 5`, `critical_stock_limit 2`, `dead_stock_days 30`,
`revenue_drop_pct 20`, `failed_payment_spike_count 3`, `stuck_order_minutes 30`,
`fulfillment_delay_hours 48`, `target_revenue_weekly 150000`,
`target_revenue_monthly 600000`, `target_revenue_yearly 7200000`.

**Commerce**
- Delivery fee **KSh 150 flat, free at subtotal ≥ KSh 600** (`fulfillment.jsx:2,62-64`),
  duplicated in Cart/Checkout/Admin [R].
- Fulfilment hours: pickup Sun–Thu 08:30–20:00, Fri 08:30–15:00, Sat closed; delivery
  Sun–Thu 08:30–16:00, Fri 08:30–15:00, Sat closed; 16:00 same-day cutoff; pickup ready
  1–2 h. **Evaluated against the visitor's device clock, not Nairobi time, and duplicated
  in five places.** [R]
- Cart quantity floor 1; decrementing below 1 removes the item; **no stock ceiling is ever
  checked**.
- Reviews: unapproved by default, public list shows approved only, average recomputed on
  approval to 1 decimal.
- Newsletter: duplicate email returns success, no error.
- Order tracking credential = order ref + last 9 digits of phone.
- Currency: KES, displayed without decimals.

**Security**
- Login 15 attempts / 15 min / IP; STK push 6/min / IP; JWT 8 h; bcrypt cost 10;
  password min length 6; body limit 5 MB; upload limit 10 MB, images only.

---

## 11. What this changes about the reform plan

1. **§12 step 3 ("back up PostgreSQL and MongoDB; export and map MongoDB catalogue data")
   is conditional on data that may not exist.** Locally `MONGODB_URI` is unset, so the
   catalogue has been served from `localStorage` mocks. Production almost certainly has
   real Mongo data, since the API 500s without it. **Confirm against the production
   databases before scheduling the migration work** — if the Mongo catalogue is empty or
   stale, steps 3, 7 and 13 collapse, and risk "Data migration defects" in §14 largely
   disappears.
2. **Phase 3 (Identity & RBAC) is a from-zero build**, not a refactor — see §6.4.
3. **The frontend cannot be used as a migration validator** while §3's fallbacks are
   active. Disable them as an explicit Phase 0/1 task.
4. **ADR-001 (Postgres only) is justified by a concrete defect**, not taste: §5.2.
5. **§18's open items are genuinely open** — order state machine (§5.6), revenue
   definition (§5.5), currency minor units (§5.7) each have contradictory answers in the
   current code.
6. **Some defects are not rewrite-blockers and should be fixed in the Express codebase
   now**, because they are live money exposure: §4.1, §4.2, §4.5, §2.2's unauthenticated
   PII reads, §6.3's SSE dev-secret fallback, §8.1's password reset. These do not need Go.

---

## 12. Suggested Phase 0 exit checklist

- [ ] Disable or visibly flag both client fallback layers; re-test the storefront against
      a stopped backend.
- [ ] Confirm whether production MongoDB holds real catalogue data (decides §11.1).
- [ ] Take and **verify** a `pg_restore`-tested backup of the live Postgres database.
- [ ] Close the unauthenticated PII reads (`/newsletter`, `/contacts`, `/order-status`).
- [ ] Verify the callback URL mismatch against Daraja sandbox logs.
- [ ] Rotate the committed credentials (`naoja@1540`, JWT secret, Daraja keys, Supabase
      secret key, Cloudinary API secret, Resend key) — all are in git history or in a
      shipped bundle.
- [ ] Delete `scripts/full-sync.js`; mark `migrations/*.sql` as non-authoritative.
- [ ] Agree the revenue-recognition rule with the business.
- [ ] Agree the order status machine and permitted transitions.
- [ ] Sign off the five-actor permission matrix against the §6.4 capability table.

---

## Appendix: verification status

Backend findings (§2, §4, §5, §6, §7, §8, §9, §10 thresholds) were read and confirmed
directly in source. Frontend findings marked **[R]** derive from an automated sweep of
`frontend/src/` and were spot-checked at `App.jsx` routing, `api.jsx:838-860` fallback
trigger, `api.jsx:750-766` hardcoded credentials, and `CheckoutPage.jsx:181-198` manual
confirmation — all four confirmed accurate. Remaining **[R]** items (analytics mock
coverage, admin tab details, fulfilment hours, field-shape drift) should be re-verified
before they drive a decision.
