-- 0004: attribute conditions on the permission catalogue (ADR-024).
--
-- The matrix's third rule (§2) is that a permission marked `[own]` is additionally constrained by
-- resource ownership: "holding the permission does not authorise acting on another customer's
-- resource". That is a second question after the permission check, and until now the answer lived
-- only in the prose of the description column — which means nothing in the database could tell an
-- ownership-scoped permission from an unscoped one, and a handler could forget the check with no
-- trace. ERD §1 forbids leaving an integrity rule to application code alone.
--
-- So the rule is declared as data: a condition *name* per permission, whose implementation is a Go
-- predicate in internal/authz. Data declares which check runs; it does not carry the check itself,
-- which is why the array is constrained to names the code actually implements.

ALTER TABLE identity.permissions
    ADD COLUMN conditions TEXT[] NOT NULL DEFAULT '{}';

-- The registry in internal/authz/conditions.go is the source of the implementations, so the
-- catalogue cannot name a condition nothing evaluates. Note the direction of this constraint: it
-- refuses a *new* condition written into the data before the code exists. The reverse — a code
-- predicate with no matching entry here — is caught by TestKnownConditionsMatchTheRegistry.
ALTER TABLE identity.permissions
    ADD CONSTRAINT permissions_conditions_are_implemented
    CHECK (conditions <@ ARRAY['owner_is_caller']::text[]);

-- ── Declare the `[own]` keys ──────────────────────────────────────────────────────────────────
--
-- Listed by key rather than derived from the description, on purpose. Deriving it here
-- (`SET conditions = ... WHERE description LIKE '%[own]%'`) would make the column a copy of a
-- sentence and the assertion below a tautology; writing the fifteen keys out is what lets the
-- assertion compare two independently-authored statements and refuse when they disagree.
UPDATE identity.permissions
   SET conditions = ARRAY['owner_is_caller']
 WHERE key IN (
    -- §3.1 Identity & Access
    'identity.account.read',
    'identity.account.update',
    'identity.account.verify',
    'identity.account.recover',
    -- §3.3 Shopping and ordering
    'shopping.cart.modify',
    'shopping.cart.read',
    'ordering.orders.create',
    'ordering.orders.read',
    'ordering.orders.cancel',
    -- §3.4 Payments
    'payments.initiate',
    'payments.status.read',
    -- §3.5 Fulfilment
    'fulfilment.delivery.read',
    'fulfilment.instructions.provide',
    -- §3.6 Engagement
    'engagement.notifications.read',
    'engagement.notifications.preferences'
 );

-- ── Seed integrity ────────────────────────────────────────────────────────────────────────────
--
-- Prose and policy are authored twice — the description in 0003, this array here — and two copies
-- of a rule is how they come to disagree. This block makes the disagreement fatal at migration
-- time instead of silent at request time, in both directions:
--
--   * declared but undocumented  — an ownership check nobody asked for;
--   * documented but undeclared  — the `[own]` in a description that promises a check no one
--                                 wrote, which is the case that actually leaks data.
--
-- The count is pinned because a key renamed or dropped from 0003's seed would otherwise pass both
-- checks while quietly removing an ownership rule from the system.
DO $$
DECLARE
    declared_only     TEXT;
    documented_only   TEXT;
    n_conditional     INTEGER;
    n_expected        INTEGER := 15;
BEGIN
    SELECT string_agg(key, ', ' ORDER BY key) INTO declared_only
      FROM identity.permissions
     WHERE conditions <> '{}'
       AND position('[own]' in description) = 0;
    IF declared_only IS NOT NULL THEN
        RAISE EXCEPTION 'these permissions declare a condition but are not marked [own]: %', declared_only;
    END IF;

    SELECT string_agg(key, ', ' ORDER BY key) INTO documented_only
      FROM identity.permissions
     WHERE position('[own]' in description) > 0
       AND conditions = '{}';
    IF documented_only IS NOT NULL THEN
        RAISE EXCEPTION 'these permissions are marked [own] but declare no condition, so the ownership rule is prose only: %', documented_only;
    END IF;

    SELECT count(*) INTO n_conditional
      FROM identity.permissions
     WHERE conditions <> '{}';
    IF n_conditional <> n_expected THEN
        RAISE EXCEPTION '% permissions carry a condition, expected % (matrix §3 marks 15 keys [own])',
            n_conditional, n_expected;
    END IF;
END;
$$;
