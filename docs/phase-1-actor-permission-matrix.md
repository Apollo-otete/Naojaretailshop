# Phase 1 — Five-Actor Permission Matrix and Role User Stories

**Derived from:** `Naoja_Retail_Shop_Architecture_and_Reform_Decisions.pdf` §3 (actors, role
boundaries, permission design), §4 (functional scope, action catalogue, action
specification standard), §7 (bounded contexts), §10 (security), §15 (definition of done),
§17 (ADR-013).
**Status:** Draft for approval — §19 step 2 requires this to be **approved** before Phase 3
implementation begins.
**Governing rule:** ADR-013 — granular backend RBAC; frontend guards are UX only.

---

## 1. Actors

Five principal actors, exactly as §3 defines them. No additional roles are introduced.

| Actor | Authenticated | Primary responsibility |
|---|---|---|
| **Guest** | No | Unauthenticated visitor |
| **Customer** | Yes | Registered buyer |
| **Admin 2 — Operations Admin** | Yes | Day-to-day store operations |
| **Admin 1 — Business Admin** | Yes | Business supervision and team coordination |
| **Super Admin** | Yes | Platform and security ownership |

### Explicitly not created (§3 role boundaries)

No accountant, maintenance officer, security officer, receptionist, auditor, guest-account
role, or second super-admin-like role. **Guest is a public actor, not an authenticated
account role.** Adding any of these requires a documented business requirement, recorded as
a new ADR.

---

## 2. Permission naming and structure

Permissions are `entity.action`, lowercase, dot-separated, namespaced by the §7 bounded
context they belong to. Roles are **collections of permissions**; a user receives
permissions only through a role, except for exceptional direct grants (see §6).

Three rules apply to every protected permission and are not repeated per row:

1. **Deny by default.** An unlisted permission is refused.
2. **Backend enforcement.** Every protected operation is authorised in a Go use case or
   middleware. Hiding a button is never the control.
3. **Ownership scope.** Permissions marked `[own]` are additionally constrained by resource
   ownership — holding the permission does not authorise acting on another customer's
   resource. §3 requires resource ownership/scope validation explicitly.

---

## 3. Permission matrix

### 3.1 Identity & Access

| Permission | Meaning | Guest | Customer | Admin 2 | Admin 1 | Super |
|---|---|:--:|:--:|:--:|:--:|:--:|
| `identity.account.read` | Read own profile `[own]` | — | ✓ | — | — | — |
| `identity.account.update` | Update own details, contact, notification preferences `[own]` | — | ✓ | — | — | — |
| `identity.account.verify` | Complete email/phone verification `[own]` | — | ✓ | — | — | — |
| `identity.account.recover` | Request password recovery `[own]` | ✓ | ✓ | — | — | — |
| `identity.customers.read` | Read customer records | — | — | ✓* | ✓ | ✓ |
| `identity.customers.suspend` | Suspend or reactivate a customer account | — | — | — | ✓ | ✓ |
| `identity.admins.read` | Read Admin 2 accounts | — | — | — | ✓ | ✓ |
| `identity.admins.manage` | Create, invite, activate, suspend Admin 2 accounts | — | — | — | ✓ | ✓† |
| `identity.super_admins.manage` | Create, invite, activate, suspend Admin 1 accounts | — | — | — | — | ✓ |
| `identity.roles.read` | Read role and permission definitions | — | — | — | ✓ | ✓ |
| `identity.roles.manage` | Create or change role **definitions** | — | — | — | — | ✓ |
| `identity.permissions.assign_scope` | Assign responsibilities / operational scope to an Admin 2 | — | — | — | ✓ | ✓ |
| `identity.permissions.assign_exceptional` | Direct permission grant outside a role | — | — | — | — | ✓ |
| `identity.sessions.revoke` | Revoke another user's sessions | — | — | — | — | ✓ |

`*` Admin 2 reads customer records **only as needed to fulfil orders** — field minimisation
applies, see §5.4.
`†` Super Admin may manage Admin 2 accounts **only where necessary for platform
governance**; routine Admin 2 management is Admin 1's responsibility. §4 grants both, but
§3's boundary rule means Admin 2 account creation should originate from Admin 1.

### 3.2 Catalogue

| Permission | Meaning | Guest | Customer | Admin 2 | Admin 1 | Super |
|---|---|:--:|:--:|:--:|:--:|:--:|
| `catalogue.products.read.published` | Browse and search published products | ✓ | ✓ | ✓ | ✓ | ✓ |
| `catalogue.products.read.draft` | Read unpublished/archived products | — | — | ✓ | ✓ | ✓ |
| `catalogue.products.create` | Create a product listing | — | — | ✓ | — | ✓ |
| `catalogue.products.update` | Edit title, description, specifications, images | — | — | ✓ | — | ✓ |
| `catalogue.products.update_price` | Change price | — | — | ✓‡ | ✓ | ✓ |
| `catalogue.products.publish` | Make visible on storefront | — | — | ✓ | — | ✓ |
| `catalogue.products.unpublish` | Remove from storefront, retain record | — | — | ✓ | — | ✓ |
| `catalogue.products.archive` | Archive a listing | — | — | ✓ | — | ✓ |
| `catalogue.products.delete` | Destroy a listing | — | — | — | ✓ | ✓ |
| `catalogue.categories.read` | Read categories | ✓ | ✓ | ✓ | ✓ | ✓ |
| `catalogue.categories.manage` | Create, update, reorder categories | — | — | ✓ | — | ✓ |
| `catalogue.categories.delete` | Delete a category | — | — | — | ✓ | ✓ |
| `catalogue.product_images.manage` | Upload and attach images | — | — | ✓ | — | ✓ |

`‡` Resolved by **ADR-016**: Admin 2 may set prices, but every change requires a reason, is
audited with previous and new value, and applies prospectively only — order lines carry a
price snapshot that a later edit can never alter.

### 3.3 Shopping, Ordering and Inventory

| Permission | Meaning | Guest | Customer | Admin 2 | Admin 1 | Super |
|---|---|:--:|:--:|:--:|:--:|:--:|
| `shopping.cart.modify` | Create and change a cart `[own]` | ✓ | ✓ | — | — | — |
| `shopping.cart.read` | Read own cart `[own]` | ✓ | ✓ | — | — | — |
| `ordering.orders.create` | Place an order from own cart `[own]` | — | ✓ | — | — | — |
| `ordering.orders.read` | Read own orders and fulfilment updates `[own]` | — | ✓ | — | — | — |
| `ordering.orders.track` | Track an order by reference + verified contact | ✓ | ✓ | — | — | — |
| `ordering.orders.cancel` | Cancel own order where the state machine allows `[own]` | — | ✓ | — | — | — |
| `ordering.orders.read.all` | Read all orders | — | — | ✓* | ✓ | ✓ |
| `ordering.orders.process` | Advance an order through permitted statuses | — | — | ✓ | — | ✓ |
| `ordering.orders.status.override` | Move an order outside its normal transition path | — | — | — | ✓ | ✓ |
| `inventory.stock.read` | Read stock levels and alerts | — | — | ✓ | ✓ | ✓ |
| `inventory.stock.adjust` | Change stock with reason, quantity, actor, timestamp | — | — | ✓ | — | ✓ |
| `inventory.movements.read` | Read append-only stock movement history | — | — | ✓ | ✓ | ✓ |
| `inventory.reservations.read` | Read active stock reservations | — | — | ✓ | ✓ | ✓ |

### 3.4 Payments

| Permission | Meaning | Guest | Customer | Admin 2 | Admin 1 | Super |
|---|---|:--:|:--:|:--:|:--:|:--:|
| `payments.initiate` | Trigger a Daraja STK Push for own order `[own]` | — | ✓ | — | — | — |
| `payments.status.read` | Read payment state of own order `[own]` | — | ✓ | ✓ | ✓ | ✓ |
| `payments.reconcile` | Reconcile a payment against Daraja records | — | — | ✓ | ✓ | ✓ |
| `payments.status.override` | Manually set an order's payment status | — | — | — | ✓ | ✓ |
| `payments.records.read.all` | Read all payment transactions | — | — | ✓* | ✓ | ✓ |
| `payments.refund.request` | Open a refund request with reason and amount | — | — | ✓ | ✓ | ✓ |
| `payments.refund` | Approve a request and record its execution | — | — | — | ✓ | ✓ |
| `payments.callback.receive` | Daraja callback — **system credential, never a user role** | — | — | — | — | — |

`payments.status.override` is deliberately **not** an Admin 2 permission. The current
implementation lets any admin session flip `payment_status` from a dropdown with no reason
recorded; §15 requires Admin 2 to be unable to manage privileged state by default.
`payments.refund` is resolved by **ADR-020**: refunds are in launch scope, executed
manually outside the application, and recorded authoritatively. Splitting request from
approval is what lets an Admin 2 raise a refund without being able to authorise one.
No actor may hold `payments.callback.receive`; it is authenticated by Daraja callback
verification, not by a role.

### 3.5 Fulfilment

| Permission | Meaning | Guest | Customer | Admin 2 | Admin 1 | Super |
|---|---|:--:|:--:|:--:|:--:|:--:|
| `fulfilment.delivery.read` | Read own delivery status `[own]` | — | ✓ | — | — | — |
| `fulfilment.instructions.provide` | Provide or select delivery/contact details at checkout `[own]` | — | ✓ | — | — | — |
| `fulfilment.assignments.read` | Read all deliveries and assignments | — | — | ✓ | ✓ | ✓ |
| `fulfilment.assign` | Assign a delivery to a person or route | — | — | ✓ | — | ✓ |
| `fulfilment.update` | Record dispatch, pickup-ready, delivered events | — | — | ✓ | — | ✓ |
| `fulfilment.config.manage` | Set service areas, fees, pickup windows | — | — | — | ✓ | ✓ |

### 3.6 Customer Engagement, Promotions, Analytics, Administration

| Permission | Meaning | Guest | Customer | Admin 2 | Admin 1 | Super |
|---|---|:--:|:--:|:--:|:--:|:--:|
| `engagement.reviews.create` | Submit a review, subject to eligibility `[own+verified purchase]` | — | ✓ | — | — | — |
| `engagement.reviews.read.published` | Read approved reviews | ✓ | ✓ | ✓ | ✓ | ✓ |
| `engagement.reviews.moderate` | Approve or reject a review | — | — | ✓ | — | ✓ |
| `engagement.notifications.read` | Read own notifications `[own]` | — | ✓ | — | — | — |
| `engagement.notifications.preferences` | Set own notification preferences `[own]` | — | ✓ | — | — | — |
| `engagement.subscribers.read` | Read newsletter subscriber list | — | — | — | ✓ | ✓ |
| `engagement.inquiries.read` | Read contact inquiries | — | — | ✓ | ✓ | ✓ |
| `engagement.inquiries.respond` | Mark or respond to an inquiry | — | — | ✓ | — | ✓ |
| `promotions.campaigns.read` | Read campaigns and eligibility rules | — | — | ✓ | ✓ | ✓ |
| `promotions.campaigns.manage` | Manage an **approved** promotion | — | — | ✓§ | ✓ | ✓ |
| `promotions.campaigns.approve` | Approve a promotion for use | — | — | — | ✓ | ✓ |
| `analytics.dashboards.read.operational` | Operational KPIs, live feed, alerts, funnel | — | — | ✓ | ✓ | ✓ |
| `analytics.dashboards.read.business` | Revenue, trends, payment health, business reports | — | — | — | ✓ | ✓ |
| `analytics.reports.export` | Export permitted reports | — | — | ✓* | ✓ | ✓ |
| `analytics.forecasts.read` | Read forecast runs | — | — | — | ✓ | ✓ |
| `analytics.targets.manage` | Set revenue and order targets | — | — | — | ✓ | ✓ |
| `administration.audit_logs.read` | Read audit and security event log | — | — | — | ✓† | ✓ |
| `administration.system_settings.manage` | Global configuration | — | — | — | — | ✓ |
| `administration.integrations.manage` | Daraja, email, storage, SMS credentials and health | — | — | — | — | ✓ |
| `administration.integration_health.read` | Read integration status | — | — | ✓ | ✓ | ✓ |
| `administration.platform_health.read` | System-wide health dashboard | — | — | — | ✓ | ✓ |

`§` Admin 2 holds `promotions.campaigns.manage` only **if that permission is granted** —
§4 lists it conditionally. Model it as absent from the default Admin 2 role and available
as an exceptional grant.
`†` Admin 1 may read the audit log **including Admin 2 activity** (§4: "Review Admin 2
activity"), but §3's boundary rule means the log itself is a Super Admin-owned artefact:
Admin 1 gets read scope over Admin 2 records, not global read.

---

## 4. Role boundary assertions

These are the negative tests that make the matrix real. Each must pass as a backend
authorisation test before Phase 3 exit (§15: "automated tests cover critical user
journeys"; §10 testing expectations: privilege escalation, IDOR/resource scope).

| # | Assertion |
|---|---|
| B1 | Admin 2 **cannot** create, invite, suspend or delete any admin account. |
| B2 | Admin 2 **cannot** change a role definition or hold `identity.roles.manage`. |
| B3 | Admin 2 **cannot** set `payment_status` directly, even on an order they can process. |
| B4 | Admin 2 **cannot** read revenue aggregates, forecasts, targets or business reports. |
| B5 | Admin 2 **cannot** change global settings or integration credentials. |
| B6 | Admin 2 **cannot** read the audit log. |
| B7 | Admin 1 **cannot** create, suspend or edit a Super Admin. |
| B8 | Admin 1 **cannot** change global role definitions or security configuration. |
| B9 | Admin 1 **cannot** modify `system_settings` or integration credentials. |
| B10 | Customer A **cannot** read, cancel, or track-to-detail Customer B's order, even with a valid token and a guessed order reference. |
| B11 | A suspended user, or a user whose permission was revoked, **fails on the next request** — not only on the next login. (§4 administrative lifecycle step 5.) |
| B12 | Removing a permission from a role removes it from every member without requiring re-login. |
| B13 | Hiding a frontend control does **not** substitute for a backend denial; every B-assertion is tested at the API, not the UI. |
| B14 | An invitation accepted by a wrong recipient cannot yield elevated permissions — activation requires the invited identifier to match. |
| B15 | Admin 2 may **request** a refund but may never approve or record its execution (ADR-020). |
| B16 | No order transition outside the ADR-015 table is reachable by any role without `ordering.orders.status.override`, and every use of it records a reason. |
| B17 | A price edit by Admin 2 does not change the total of any order placed before it (ADR-016). |

---

## 5. User stories by role

Format: story with acceptance criteria. §4's action specification standard is applied in
full to the important actions in §6 below.

### 5.1 Guest

- **G1 — Browse without an account.** As a guest I can open the storefront, browse
  categories and search products so that I can evaluate a purchase without registering.
  *AC:* catalogue renders with no session; unpublished products are absent; a guest can
  build and modify a cart; cart contents survive navigation but are not required to survive
  a different device.
- **G2 — Inspect a product.** As a guest I can see price, images, availability and
  specifications. *AC:* availability reflects real stock, not a static label; views are
  recorded.
- **G3 — Track an existing order.** As a guest I can check an order using its reference and
  the contact details I supplied. *AC:* requires both factors; returns only status and
  fulfilment information, never full address or payment records; rate-limited; cannot be
  iterated to enumerate references.
- **G4 — Start an account.** As a guest I can register and then verify my contact details.
  *AC:* registration alone does not grant `ordering.orders.create` until verification, if
  verification is required by the §18 auth decision.

### 5.2 Customer

- **C1 — Log in and recover access.** *AC:* session expiry is communicated; refresh rotation
  and revocation work; recovery does not disclose whether an email exists.
- **C2 — Checkout with an authoritative total.** As a customer the total I am charged is
  calculated by the backend from server-side prices. *AC:* client-submitted totals are
  ignored; cart contents, stock and delivery details are revalidated at submission; a
  changed price between cart and checkout is surfaced, not silently applied.
- **C3 — Pay by M-Pesa and see the truth.** *AC:* STK Push is initiated against a server-side
  order; pending, failed, cancelled and successful states are each distinguishable; the
  success message the customer sees is derived from the verified callback, never from a
  redirect; the amount actually paid is compared to the amount owed.
- **C4 — Never be oversold.** *AC:* stock is reserved transactionally at order creation; two
  concurrent customers cannot both confirm the last unit; an expired or failed reservation
  releases stock.
- **C5 — View my order history and tracking.** *AC:* only own orders; fulfilment updates
  visible; no access to another customer's record.
- **C6 — Review something I actually bought.** *AC:* per **ADR-017** — authenticated
  customer, a delivered order containing that product, within 90 days of delivery, one
  review per product per customer; eligibility computed server-side from order records; a
  review does not appear publicly until approved; the unauthenticated review endpoint is
  removed.
- **C7 — Control my details and notifications.** *AC:* profile, contact and notification
  preferences editable; data retention and deletion requests possible (§18 open item).

### 5.3 Admin 2 — Operations Admin

- **O1 — Run the store day to day.** *AC:* can create, edit, publish, unpublish, archive
  products and manage categories and images within granted scope.
- **O2 — Process orders through defined statuses.** *AC:* can advance status only along
  permitted transitions; every change records actor, timestamp, previous and new state.
- **O3 — Keep stock accurate and traceable.** *AC:* every adjustment requires a reason;
  stock movement history is append-only; corrections are compensating entries, never edits.
- **O4 — See and act on alerts.** *AC:* low-stock and out-of-stock alerts visible and
  acknowledgeable; operational reports exportable within scope.
- **O5 — Handle fulfilment.** *AC:* can assign and update deliveries, record pickup-ready
  and delivered events.
- **O6 — Moderate reviews and inquiries.** *AC:* approve/reject with record; respond to
  contact inquiries.
- **O7 — See only what fulfilment needs.** *AC:* customer records shown with field
  minimisation; no revenue, no payment overrides, no admin management, no audit log.

### 5.4 Admin 1 — Business Admin

- **B1 — Supervise the business.** *AC:* dashboard of KPIs, revenue, sales trends, payment
  health; management reports and analytics accessible.
- **B2 — Build and govern the operations team.** *AC:* create, invite, activate, suspend
  Admin 2 accounts; assign responsibilities and operational scope; cannot exercise Super
  Admin controls.
- **B3 — Review Admin 2 activity.** *AC:* audit log read scope over Admin 2 actions;
  operational performance reviewable; escalation path for business-level alerts.
- **B4 — Own the sensitive levers.** *AC:* sole non-Super holder of payment status override,
  price authority, promotion approval, customer suspension, target setting, delivery
  configuration.
- **B5 — Not become a system owner.** *AC:* cannot change role definitions, global settings,
  security configuration or integration credentials.

### 5.5 Super Admin

- **S1 — Own the platform and its access model.** *AC:* create and manage Admin 1 accounts;
  create and maintain role definitions and granular permissions; manage platform users and
  privileged access.
- **S2 — Own security and audit.** *AC:* inspect audit logs and security events; revoke
  sessions; no other actor can alter the log.
- **S3 — Own integrations and configuration.** *AC:* manage global settings and integration
  credentials; monitor Daraja, email, storage, background jobs and integration health.
- **S4 — Be tightly controlled.** *AC:* Super Admin membership is minimal; every privileged
  action audited; §3's requirement that this role is used only for platform-level
  administration is testable by the B-assertions.

---

## 6. Action specifications (§4 standard)

§4 requires every important action to carry: action ID, actor, purpose, required permission,
preconditions, input fields, validation rules, application behaviour, success result,
failure cases, audit requirement, acceptance test. Applied below to the actions where
correctness is not obvious. Routine reads and simple CRUD get the same template during
Phase 1 review.

### A-001 · Place order
- **Actor:** Customer · **Permission:** `ordering.orders.create`
- **Preconditions:** authenticated; verified contact per policy; cart non-empty; every line
  in stock; delivery details valid for service area.
- **Input:** cart reference, fulfilment method (`pickup|delivery`), delivery address or
  pickup selection, contact name/phone/email, optional notes.
- **Validation:** server recalculates every price and total from persisted product records;
  client-submitted amounts ignored; cart contents revalidated against current catalogue;
  stock checked inside the transaction.
- **Behaviour:** one database transaction — create order + order lines, reserve stock,
  compute total, generate a collision-safe unique reference.
- **Success:** order in `pending` with `payment_status=pending`; reservation recorded with
  expiry; reference returned.
- **Failures:** insufficient stock (which line), price changed since cart, invalid area,
  empty cart, expired session.
- **Audit:** order created, actor, total, lines, reservation ID.
- **Acceptance test:** two concurrent customers with one unit in stock — exactly one order
  succeeds; the other receives a stock failure; no negative stock; no duplicate reference.

### A-002 · Initiate M-Pesa STK Push
- **Actor:** Customer · **Permission:** `payments.initiate` `[own]`
- **Preconditions:** order belongs to caller; order payable; not already paid; not
  over rate limit.
- **Input:** order reference.
- **Validation:** amount read from the **server-side order**, never from the request; phone
  normalised to `254XXXXXXXXX`; ownership proven by session, not by reference possession.
- **Behaviour:** call Daraja; persist `CheckoutRequestID` against the order; record a
  payment attempt row.
- **Success:** prompt sent; attempt recorded; UI shows pending, **not** paid.
- **Failures:** Daraja auth error, invalid phone, order not owned, already paid, rate limit.
- **Audit:** attempt ID, order, amount requested, Daraja response code.
- **Acceptance test:** a request with a tampered `amount` is charged the server total; a
  request for another customer's order is refused.

### A-003 · Receive and apply Daraja callback
- **Actor:** system (Safaricom) · **Permission:** `payments.callback.receive` — never a role
- **Preconditions:** payload structurally valid; source verified per §11 (webhooks only
  through verified backend endpoints).
- **Input:** `Body.stkCallback` — `ResultCode`, `ResultDesc`, `CheckoutRequestID`,
  `CallbackMetadata` (`MpesaReceiptNumber`, `PhoneNumber`, `Amount`).
- **Validation:** locate order **by `CheckoutRequestID` only**; confirm the order is still
  `pending`; **compare `Amount` to the order total and reject or flag a mismatch**; never
  match by phone suffix.
- **Behaviour:** idempotent state transition guarded by current payment state, inside a
  transaction that commits inventory effects **exactly once**; then notify.
- **Success:** `payment_status=paid`, transaction ID stored, order moves to the next state,
  reservation converted to committed stock movement.
- **Failures:** unknown `CheckoutRequestID`, amount mismatch, already settled (no-op, still
  ACK), malformed payload.
- **Audit:** callback received, order, amount expected vs paid, transition applied or
  refused, raw reference retained.
- **Acceptance test:** the same callback delivered five times settles the order once and
  sends one confirmation; a callback for KES 1 against a KES 10,000 order does **not** mark
  it paid; a forged callback from an unverified source is refused; two orders sharing a
  phone suffix are never confused.

### A-004 · Manually override payment status
- **Actor:** Admin 1 or Super Admin · **Permission:** `payments.status.override`
- **Preconditions:** order exists; current state known; reason supplied.
- **Input:** order ID, new status, free-text reason.
- **Validation:** Admin 2 token is refused even with a valid session; reason mandatory;
  transition permitted by the state machine.
- **Behaviour:** apply transition; write stock effects only if the transition requires them,
  guarded against repetition.
- **Success:** status changed, prior state recoverable from the log.
- **Failures:** missing reason, insufficient role, illegal transition.
- **Audit:** actor, timestamp, order, previous value, new value, reason — mandatory.
- **Acceptance test:** assertion B3 passes; an override without a reason is rejected.

### A-005 · Adjust stock
- **Actor:** Admin 2 · **Permission:** `inventory.stock.adjust`
- **Input:** product, delta or new absolute quantity, reason category, note.
- **Validation:** product exists; result cannot be negative; reason mandatory; adjustment
  recorded as an append-only movement, never an in-place edit of history.
- **Success:** new level, movement row with actor/timestamp/reason.
- **Failures:** unknown product, negative result, missing reason, concurrent modification.
- **Audit:** mandatory.
- **Acceptance test:** correcting a mistake produces a second compensating movement; the
  original remains visible.

### A-006 · Cancel an order
- **Actor:** Customer (`ordering.orders.cancel`, state-limited) or Admin 2
  (`ordering.orders.process`)
- **Validation:** cancellation permitted only from states the §18 state machine defines;
  releasing stock is guarded so it happens once.
- **Behaviour:** transaction — mark cancelled, release reservation/committed stock, record
  movement.
- **Failure cases:** already delivered, already cancelled (no-op), already refunded.
- **Audit:** actor, reason, stock released.
- **Acceptance test:** cancelling the same order twice does **not** add stock twice.

### A-007 · Create an Admin 2 account
- **Actor:** Admin 1 · **Permission:** `identity.admins.manage`
- **Input:** name, email, role assignment, optional operational scope.
- **Validation:** cannot assign a role containing permissions Admin 1 does not hold
  (no privilege ladder-climbing); cannot create a Super Admin; email unique.
- **Behaviour:** invitation created, not an active account; token issued out-of-band;
  activation requires verification.
- **Success:** pending invitation; no usable session until activated.
- **Failures:** elevated role requested, duplicate email, insufficient permission.
- **Audit:** inviter, invitee email, role granted, scope, activation timestamp.
- **Acceptance test:** Admin 1 attempting to grant `identity.roles.manage` to a new Admin 2
  is refused (supports B1/B2/B7).

### A-008 · Change a user's role or revoke a permission
- **Actor:** Super Admin (`identity.roles.manage`, `identity.permissions.assign_exceptional`)
  or Admin 1 within scope (`identity.permissions.assign_scope`)
- **Validation:** change must not silently widen privileges beyond the actor's own grant;
  exceptional direct grants must be individually justified and are always audited.
- **Behaviour:** effective permissions recomputed on the next authorisation check, not
  cached for the session lifetime.
- **Audit:** actor, subject, before/after, reason.
- **Acceptance test:** assertions B11 and B12 pass.

---

## 7. Resolution of the §18 items that blocked this matrix

Resolved in `docs/adr/ADR-015-020-open-item-resolutions.md`. Recorded as **Proposed**,
pending product-owner ratification where marked.

| # | Question | Resolution | Still needs |
|---|---|---|---|
| 7.1 | Order statuses and permitted transitions | **ADR-015** — 10-state order machine + independent payment machine, explicit transition table, `shipped` retired for `dispatched`, pickup/delivery paths separated | — |
| 7.2 | May Admin 2 set prices? | **ADR-016** — yes, with mandatory reason, audit, prospective-only effect | **RATIFY** |
| 7.3 | Review eligibility rule | **ADR-017** — authenticated customer, delivered order containing the product, within 90 days, one per product, still moderated | blocked on Phase 3 identity; disable public endpoint until then |
| 7.4 | Delivery areas, fees, windows, authority | **ADR-018** — zone model, fee/threshold moved into `system_settings`, server-side hours in `Africa/Nairobi`, Admin 1 configures / Admin 2 executes | **RATIFY** zone coverage and amounts |
| 7.5 | Authentication mechanism | **ADR-019** — server-side sessions, HttpOnly cookie, Argon2id, immediate revocation, no tokens in query strings | verify cookie behaviour against the real deployment |
| 7.6 | Refunds in launch scope? | **ADR-020** — yes; requested by Admin 2, approved by Admin 1/Super, executed manually, recorded; no auto-restock | **RATIFY** (cash movement) |

Still unresolved from §18 and not answerable by engineering: **product variants** (deferred
by ADR-020's companion note; `sku` uniqueness is added now), **promotions/discount
precedence**, **tax/invoice requirements**, **email/SMS/WhatsApp provider choice and
budget**, **data retention and Kenyan privacy obligations**, and **production hosting,
domain, TLS and backup arrangements**.

---

## 8. Consequences for the next §19 steps

This matrix feeds directly into §19 step 3 (domain model and ERD). It requires at minimum:

- `users`, `roles`, `permissions`, `user_roles`, `role_permissions`, plus **direct
  `user_permissions` for exceptional grants**, each with grantor, reason and timestamp
  (§3: exceptional assignment explicitly audited).
- `invitations` with status and activation record (§4 lifecycle step 3).
- `sessions` with rotation and revocation (§10).
- `audit_logs` as append-only, actor-attributed, covering every mandatory-audit action above
  (§9 data rule: correct mistakes with compensating entries).
- `stock_movements` and `stock_reservations` as first-class tables — the current schema has
  neither, and A-001/A-005/A-006 depend on them.
- A `payments`/`payment_transactions` table separate from `orders`, because A-003 requires
  per-attempt idempotency that a single `payment_status` column on `orders` cannot express.

And into §19 step 4 (OpenAPI): every protected operation needs its permission declared in
the contract, and 403 responses with a consistent error shape. Both are satisfied in
`contracts/openapi.yaml` — each operation carries `x-permissions` naming the keys above,
`x-scoped: own` marks the ownership-scoped ones, and the shared `Error`/`PermissionError`
shape is the single error contract for every failure.

---

## 9. Approval

| Role | Name | Date | Decision |
|---|---|---|---|
| Product owner | | | ☐ approve ☐ changes |
| Super Admin (technical) | | | ☐ approve ☐ changes |

Approval closes §19 step 2 and unblocks Phase 3 (§13 exit criterion: "role lifecycle and
authorization tests pass"). §7 items must be resolved or explicitly deferred with a
recorded decision.
