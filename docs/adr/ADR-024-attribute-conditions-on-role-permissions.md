# ADR-024: Attribute conditions as a second gate on role permissions

- **Status:** Proposed — the mechanism is decided and buildable; two items below are open and
  neither blocks Phase 3:
  1. **Which permissions carry a condition beyond `[own]`.** The 15 ownership keys are declared
     now. The business conditions in ADR-015 (a transition is only permitted from a state the
     order is actually in), ADR-016 (a price change requires a reason and is prospective only)
     and ADR-020 (a refund requested by Admin 2 is never executed by Admin 2) are enforced in
     the use cases that own those rules, not as declared conditions, until a second reader of
     the same rule exists.
  2. **Who may change a condition.** Nothing may, yet: the column is written by migration only.
     A `identity.permissions.manage_conditions` key is deliberately *not* seeded, because a
     permission to weaken an ownership rule is a permission whose own scope is the question this
     ADR has not settled.

- **Date:** 2026-11-05

## Context

The matrix (§2, rule 3) adds a third rule to every protected permission: the keys marked `[own]`
are *additionally* constrained by resource ownership — "holding the permission does not authorise
acting on another customer's resource". §3 requires that validation explicitly, and 15 of the 75
seeded keys are marked that way (`identity.account.*`, `shopping.cart.*`, `ordering.orders.*`
read/create/cancel, `payments.initiate`/`status.read`, `fulfilment.delivery.read`,
`fulfilment.instructions.provide`, `engagement.notifications.*`).

A permission check alone cannot express that. `ordering.orders.read` is a yes/no fact about a
role; "and only your own order" is a fact about the *request*. So the evaluator needs a second
input: which resource is being touched, and who owns it.

Two things constrain the answer. **ERD §1** forbids leaving an integrity rule to application
code alone, so a rule like this should be *declared* somewhere a query can read it rather than
scattered through handlers. And **§8** forbids business rules in the Gin layer, so whatever
decides this lives behind `port.Authorizer`, not in a middleware that knows URLs.

## Decision

**Attributes are a second gate, evaluated in Go, with the gate's *name* declared as data in
PostgreSQL.** Three pieces:

1. `identity.permissions.conditions TEXT[]` — one entry per permission, e.g.
   `{owner_is_caller}` for the `[own]` keys, `{}` for everything else. Migration 0004 also
   asserts, in the database, that the set of keys with a non-empty condition array is exactly the
   set whose description contains `[own]` — so the prose and the policy cannot drift apart
   silently.
2. A **closed registry** of condition names in `internal/authz`. One implementation today:
   `owner_is_caller`, satisfied when the request's `Resource.OwnerID` equals the principal's user
   id. An admin acting *on* a customer (suspend, read, revoke sessions) holds those keys without
   `[own]`, so ownership never applies to them — which is what the matrix's separate Admin rows
   mean.
3. `port.Authorizer` gains a request object: `Authorize(ctx, Request{Permissions, Resource})`.
   `Permissions` is the contract's `x-permissions` list, all of which must be held; `Resource`
   names what is being touched. There is **no** free-form `map[string]any` of attributes: an
   untyped bag turns every typo into a silent deny, and every new rule into a caller that has to
   guess the key.

Order of evaluation, and it is not negotiable: **authenticate → RBAC permission check → attribute
conditions → the use case's own rules.** A condition can only ever narrow an allowed request. It
can never grant one, so a missing role permission cannot be compensated for by an ownership
match. Deny by default (§10) is preserved at every step, and an unrecognised condition name in
the database denies rather than passes.

Conditions reach the decision the same way permissions do: read per request, out of
`GrantReader.GrantsFor`, and attached to the `Principal`. No cache (§13 of the plan; ADR-019).
A cached snapshot is precisely the thing that would make B11 and B12 false.

## Alternatives considered

- **A policy interpreter** — conditions stored as expressions (`owner_id == caller.id`,
  `state == 'pending' && age < 15m`) evaluated by a small engine. Rejected: it defers the hard
  part (what the language may say, how it is typed, how it fails) into a component that is easy
  to get subtly wrong and hard to test exhaustively, for exactly one rule today. A rule that
  needs to be *expressible at runtime by an operator* is a different decision, and ADR-024's open
  item 2 is where that would be revisited.
- **An external policy engine** (OPA/Cerbos-style). Rejected on the same grounds ADR-019 rejects
  anything that adds a second source of truth for authority: it is one more system that must be
  as available as PostgreSQL for a request to be authorised, and §13's Phase 3 exit criterion is
  a test suite, not a service.
- **Ownership checks written per handler, with no declared conditions.** Rejected by ERD §1 and
  by the inventory's own history: the legacy `auth.js:122` master-key hole is what a rule that
  lives only in application code decays into. Declaring `{owner_is_caller}` in the catalogue also
  makes "which permissions are ownership-scoped" a one-line query instead of a grep.
- **ABAC instead of RBAC** — drop roles and decide everything from attributes. Rejected: the
   matrix *is* a role table, §3's delegation model is roles, and B1–B14 are asserted about roles.
   Attributes here are a narrowing layer, not a replacement.

## Consequences

- The catalogue becomes policy in a stronger sense: a permission's row now says what *shape* of
  check follows it. Anyone adding a `[own]` key to a description must also add the condition, and
  migration 0004's assertion is what fails the migration when they do not.
- Every route that names a resource has to say which one. `server.Require(keys...)` stays as the
  declaration of permissions; the resource is supplied by the handler/middleware that resolves the
  id, which is a deliberate cost — a route that cannot name its resource cannot claim ownership
  either, and gets the permission-only behaviour.
- `owner_is_caller` on a request whose resource carries no owner is a **deny**, not a pass: the
  alternative would let a missing lookup become an authorisation answer.
- Nothing about the 75 keys, the four seeded roles or the contract's `x-permissions` lists
  changes. This is additive to the first gate.
- ADR-015/016/020's business conditions are *not* moved here now (open item 1). Moving one later
  is a data change plus a registered predicate; it is not a redesign, which is why choosing the
  narrow mechanism now is cheap.

## Verification

- Unit: `owner_is_caller` admits the owner and refuses a non-owner, at the evaluator, for a
  principal that unambiguously holds the permission — so a failure cannot be a missing grant.
- Unit: an unrecognised condition name in the data denies; an empty condition list does not
  narrow anything.
- Unit: a principal with no session is refused before any condition is considered (B11), and
  removing a condition or a permission changes the next call, not the next login (B12).
- Migration: 0004's assertion is proven by *breaking* it in a test — mark one key's description
  `[own]` without declaring the condition, and expect the migration to refuse.
- API-level (task #15, Phase 3 exit): customer A reads its own order 200, customer B's order 403
  `PERMISSION_DENIED`; customer A cancels B's order 403; Admin 1 reads any customer's record 200
  because that key has no condition.

## Reopens if

a condition needs to be authored or edited at runtime by an operator (that is a policy-language
decision, and the first one), when a second context needs the same business rule in two places,
or if more than a handful of keys turn out to be conditional — at which point the registry's
"one Go function per rule" starts being read as a scaling problem rather than a safety feature.
