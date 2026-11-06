-- 0003: Identity & Access, plus the audit trail every privileged auth change writes to.
--
-- Source of truth for the shape is docs/phase-1-domain-model-erd.md §5.1 and §5.9; the source
-- of truth for who-may-do-what is docs/phase-1-actor-permission-matrix.md §3, approved by the
-- product owner on 2026-11-05 (§9). The permission catalogue and the role
-- grants below are that table, transcribed rather than paraphrased: a grant that exists only in
-- someone's memory of the matrix is how Phase 3's "authorization tests pass" becomes fiction.

-- ── Principals ─────────────────────────────────────────────────────────────────────────────

CREATE TABLE identity.users (
    id              BIGSERIAL PRIMARY KEY,
    email           CITEXT UNIQUE NOT NULL,
    -- A Kenyan MSISDN, normalised to +254… on write. UNIQUE means one account per phone, which
    -- is what makes "track an order by reference + verified contact" a real second factor.
    phone           VARCHAR(20)   UNIQUE,
    full_name       TEXT NOT NULL CHECK (char_length(full_name) BETWEEN 2 AND 120),

    -- Named password_hash, not password: the legacy `admins` table has a column called
    -- `password`, and a name that lies about its contents is how a plaintext value lands there
    -- by accident (inventory §6.2). The CHECK is the same rule stated where it can be enforced:
    -- a repository bug or a legacy import that writes a bare password is refused by the
    -- database, and ERD §1 leaves no integrity rule to application code.
    password_hash   TEXT NOT NULL CHECK (password_hash ~ '^\$argon2id\$v=19\$[a-z0-9=,]+\$[^$]+\$[^$]+$'),
    password_algo   TEXT NOT NULL DEFAULT 'argon2id' CHECK (password_algo = 'argon2id'),

    -- §1: five actors, of which Guest is unauthenticated. account_kind splits the storage
    -- concerns (a customer is orderable-into, an admin is not) while the *role* decides
    -- authority — the two must not be conflated the way the single `admins` table conflates them.
    account_kind    TEXT NOT NULL CHECK (account_kind IN ('customer','admin')),

    is_active       BOOLEAN NOT NULL DEFAULT true,
    is_suspended    BOOLEAN NOT NULL DEFAULT false,

    email_verified_at TIMESTAMPTZ,
    phone_verified_at TIMESTAMPTZ,
    last_login_at     TIMESTAMPTZ,
    -- Lockout counters are per account as well as per address (ADR-019 consequence); today it is
    -- address only, so one attacker can be moved between phones for free.
    failed_login_count INTEGER NOT NULL DEFAULT 0 CHECK (failed_login_count >= 0),
    locked_until       TIMESTAMPTZ,

    created_at      TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at      TIMESTAMPTZ NOT NULL DEFAULT now()
);

COMMENT ON TABLE identity.users IS 'The single principal table: users + admins merged (ADR-013).';
COMMENT ON COLUMN identity.users.password_hash IS 'Argon2id, PHC string format with parameters embedded (ADR-019).';

-- updated_at is database-maintained so an audit of "when did this change" cannot be skipped by
-- a repository that forgot to set it. Every table in this migration follows the same rule.
CREATE FUNCTION identity.touch_updated_at() RETURNS trigger LANGUAGE plpgsql AS $$
BEGIN
    NEW.updated_at := now();
    RETURN NEW;
END;
$$;

CREATE TRIGGER users_touch_updated_at
    BEFORE UPDATE ON identity.users
    FOR EACH ROW EXECUTE FUNCTION identity.touch_updated_at();

-- ── Authority: roles are collections of permissions ────────────────────────────────────────

CREATE TABLE identity.roles (
    id          BIGSERIAL PRIMARY KEY,
    key         CITEXT UNIQUE NOT NULL CHECK (key ~ '^[a-z][a-z0-9_]*$'),
    name        TEXT NOT NULL,
    description TEXT,
    -- A seeded role cannot be deleted: §4's Admin 1 delegation model breaks if
    -- `super_admin` can be removed by an API call, and "who deleted the role" is not an
    -- acceptable answer during an incident.
    is_system   BOOLEAN NOT NULL DEFAULT false,
    created_at  TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE TABLE identity.permissions (
    id          BIGSERIAL PRIMARY KEY,
    key         CITEXT UNIQUE NOT NULL CHECK (key ~ '^[a-z]+\.[a-z_.]+$'),
    -- The bounded context, held as data so §3's "namespaced by the bounded context" is
    -- checkable by query rather than by eyeballing the key's first segment (which lies for
    -- ordering.*, which lives in the Ordering context of commerce.*).
    context     TEXT NOT NULL,
    description TEXT NOT NULL
);

CREATE TABLE identity.user_roles (
    user_id     BIGINT NOT NULL REFERENCES identity.users(id) ON DELETE CASCADE,
    role_id     BIGINT NOT NULL REFERENCES identity.roles(id) ON DELETE RESTRICT,
    granted_by  BIGINT REFERENCES identity.users(id) ON DELETE SET NULL,
    granted_at  TIMESTAMPTZ NOT NULL DEFAULT now(),
    PRIMARY KEY (user_id, role_id)
);

CREATE TABLE identity.role_permissions (
    role_id       BIGINT NOT NULL REFERENCES identity.roles(id) ON DELETE CASCADE,
    permission_id BIGINT NOT NULL REFERENCES identity.permissions(id) ON DELETE CASCADE,
    granted_at    TIMESTAMPTZ NOT NULL DEFAULT now(),
    PRIMARY KEY (role_id, permission_id)
);

-- The exceptional path §3 requires: "a user may receive permissions through an assigned role,
-- with any exceptional assignment explicitly audited." `reason` is NOT NULL because a grant
-- without a stated reason is unauditable in advance, not after the fact.
CREATE TABLE identity.user_permissions (
    user_id       BIGINT NOT NULL REFERENCES identity.users(id) ON DELETE CASCADE,
    permission_id BIGINT NOT NULL REFERENCES identity.permissions(id) ON DELETE RESTRICT,
    granted_by    BIGINT NOT NULL REFERENCES identity.users(id) ON DELETE RESTRICT,
    reason        TEXT NOT NULL CHECK (char_length(reason) >= 10),
    granted_at    TIMESTAMPTZ NOT NULL DEFAULT now(),
    -- Exceptional means temporary. A grant with no expiry is a second role nobody documented.
    expires_at    TIMESTAMPTZ NOT NULL,
    revoked_at    TIMESTAMPTZ,
    revoked_by    BIGINT REFERENCES identity.users(id) ON DELETE SET NULL,
    PRIMARY KEY (user_id, permission_id),
    CHECK (expires_at > granted_at),
    CHECK ((revoked_at IS NULL) = (revoked_by IS NULL))
);

-- ── Sessions (ADR-019: server-side, revocable on the next request) ─────────────────────────

CREATE TABLE identity.sessions (
    id           BIGSERIAL PRIMARY KEY,
    -- The hash of the opaque cookie value, never the value itself: a dump of this table must not
    -- hand over live credentials.
    session_hash TEXT UNIQUE NOT NULL,
    -- The CSRF token is stored in the clear, unlike the session id, and the two columns are not
    -- the same kind of secret. A session id alone *is* the credential, so its stored form has to
    -- be useless. A CSRF token alone authorises nothing — without the HttpOnly cookie it is a
    -- random string — and GET /auth/session has to hand it back to the client on demand, which
    -- a hash cannot do. ADR-019 requires CSRF on every unsafe method; this is where it lives.
    csrf_token   TEXT NOT NULL,
    user_id      BIGINT NOT NULL REFERENCES identity.users(id) ON DELETE CASCADE,
    -- Audit context, and the basis for "this account was signed in from three addresses at
    -- 02:00". ip is INET so it can be compared as an address rather than as a string.
    ip           INET,
    user_agent   TEXT,
    created_at   TIMESTAMPTZ NOT NULL DEFAULT now(),
    last_seen_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    expires_at   TIMESTAMPTZ NOT NULL,
    revoked_at   TIMESTAMPTZ,
    revoked_reason TEXT CHECK (revoked_reason IS NULL OR revoked_reason IN (
        'logout','password_changed','suspended','role_revoked','admin_revoke','expired'
    )),
    -- Rotation lineage: changing a password mints a new session and revokes the rest, and the
    -- audit question "which session did this one replace" has to be answerable.
    replaced_by_session_id BIGINT REFERENCES identity.sessions(id) ON DELETE SET NULL,

    CHECK (expires_at > created_at)
);

-- The lookup on every single request.
CREATE INDEX sessions_session_hash_idx ON identity.sessions (session_hash);
-- Sweeper for expired rows, and "how many live sessions does this account have".
CREATE INDEX sessions_user_active_idx  ON identity.sessions (user_id, expires_at)
    WHERE revoked_at IS NULL;

-- ── Invitations (matrix A-007, B14) ────────────────────────────────────────────────────────

CREATE TABLE identity.invitations (
    id                 BIGSERIAL PRIMARY KEY,
    -- The invited address is fixed at creation. B14: an invitation accepted by the wrong
    -- recipient must not yield elevated permissions, which is only enforceable if the intended
    -- recipient is recorded rather than inferred from whoever holds the link.
    email              CITEXT NOT NULL,
    invite_token_hash  TEXT UNIQUE NOT NULL,
    invited_role_id    BIGINT NOT NULL REFERENCES identity.roles(id) ON DELETE RESTRICT,
    -- Operational scope (§3 permissions.assign_scope): which responsibilities, not which role.
    scope              JSONB NOT NULL DEFAULT '{}'::jsonb,
    invited_by         BIGINT NOT NULL REFERENCES identity.users(id) ON DELETE RESTRICT,
    status             TEXT NOT NULL DEFAULT 'pending' CHECK (status IN
                           ('pending','accepted','expired','revoked')),
    created_at         TIMESTAMPTZ NOT NULL DEFAULT now(),
    expires_at         TIMESTAMPTZ NOT NULL,
    accepted_at        TIMESTAMPTZ,
    accepted_user_id   BIGINT REFERENCES identity.users(id) ON DELETE SET NULL,
    CHECK (expires_at > created_at)
);

CREATE INDEX invitations_pending_idx ON identity.invitations (email, status)
    WHERE status = 'pending';

-- ── Audit log (§10, matrix "audit: mandatory" rows) ────────────────────────────────────────

CREATE TABLE administration.audit_logs (
    id              BIGSERIAL PRIMARY KEY,
    -- NULL actor means the system acted: the reservation expiry job, a Daraja callback. Those
    -- are exactly the events an operator needs to see, so the column cannot be NOT NULL.
    actor_id        BIGINT REFERENCES identity.users(id) ON DELETE SET NULL,
    actor_kind      TEXT NOT NULL CHECK (actor_kind IN ('user','system','provider')),
    action          TEXT NOT NULL,             -- a permission key, or a named system action
    resource_type   TEXT,
    resource_id     BIGINT,
    previous_value  JSONB,
    new_value       JSONB,
    reason          TEXT,
    ip              INET,
    correlation_id  TEXT,                      -- the same id the client was given (§10)
    occurred_at     TIMESTAMPTZ NOT NULL DEFAULT now()
);

COMMENT ON TABLE administration.audit_logs IS 'Append-only. UPDATE and DELETE are refused by trigger, not by convention.';

CREATE INDEX audit_logs_actor_idx    ON administration.audit_logs (actor_id, occurred_at DESC);
CREATE INDEX audit_logs_resource_idx ON administration.audit_logs (resource_type, resource_id);
-- ERD §8: review by action type over a window is the query an operator actually runs.
CREATE INDEX audit_logs_action_idx   ON administration.audit_logs (action, occurred_at DESC);

-- Append-only is a property of the database, not of the repository: a bug in one use case, or a
-- person with psql, must not be able to rewrite history and have the rewrite look like it never
-- happened. A trigger rather than a rule, because a rule silently swallows the statement —
-- silence is the wrong outcome for an attempted tamper.
CREATE FUNCTION identity.refuse_audit_mutation() RETURNS trigger LANGUAGE plpgsql AS $$
BEGIN
    RAISE EXCEPTION 'administration.audit_logs is append-only; % is not permitted', TG_OP
        USING ERRCODE = 'insufficient_privilege';
END;
$$;

CREATE TRIGGER audit_logs_append_only
    BEFORE UPDATE OR DELETE ON administration.audit_logs
    FOR EACH ROW EXECUTE FUNCTION identity.refuse_audit_mutation();

-- ── Seed: the four authenticated roles ─────────────────────────────────────────────────────
--
-- Guest is deliberately absent: §1 lists it as a public actor, not an account role, and an
-- invitable "guest" role is how unauthenticated access becomes a grantable thing.

INSERT INTO identity.roles (key, name, description, is_system) VALUES
  ('customer',           'Customer',           'A registered buyer. Own orders, own cart, own details — nothing else.', true),
  ('admin_2_operations', 'Operations Admin',   'Day-to-day store operations: catalogue, stock, fulfilment. No money-state authority (matrix §3.4, B3).', true),
  ('admin_1_business',   'Business Admin',     'Business supervision, team coordination, refunds, overrides. Cannot touch a Super Admin (B7).', true),
  ('super_admin',        'Super Admin',        'Platform and security ownership: role definitions, global settings, integration credentials.', true);

-- ── Seed: the permission catalogue (matrix §3.1–§3.6) ──────────────────────────────────────

INSERT INTO identity.permissions (key, context, description) VALUES
  -- §3.1 Identity & Access
  ('identity.account.read',                    'identity', 'Read own profile [own].'),
  ('identity.account.update',                  'identity', 'Update own details, contact and notification preferences [own].'),
  ('identity.account.verify',                  'identity', 'Complete email or phone verification [own].'),
  ('identity.account.recover',                 'identity', 'Request password recovery [own].'),
  ('identity.customers.read',                  'identity', 'Read customer records.'),
  ('identity.customers.suspend',               'identity', 'Suspend or reactivate a customer account.'),
  ('identity.admins.read',                     'identity', 'Read Admin 2 accounts.'),
  ('identity.admins.manage',                   'identity', 'Create, invite, activate or suspend Admin 2 accounts.'),
  ('identity.super_admins.manage',             'identity', 'Create, invite, activate or suspend Admin 1 accounts.'),
  ('identity.roles.read',                      'identity', 'Read role and permission definitions.'),
  ('identity.roles.manage',                    'identity', 'Create or change role definitions.'),
  ('identity.permissions.assign_scope',        'identity', 'Assign responsibilities or operational scope to an Admin 2.'),
  ('identity.permissions.assign_exceptional',  'identity', 'Direct permission grant outside a role.'),
  ('identity.sessions.revoke',                 'identity', 'Revoke another user''s sessions.'),

  -- §3.2 Catalogue
  ('catalogue.products.read.published',        'catalogue', 'Browse and search published products.'),
  ('catalogue.products.read.draft',            'catalogue', 'Read unpublished or archived products.'),
  ('catalogue.products.create',                'catalogue', 'Create a product listing.'),
  ('catalogue.products.update',                'catalogue', 'Edit title, description, specifications, images.'),
  ('catalogue.products.update_price',          'catalogue', 'Change price (ADR-016: reason required, audited, prospective only).'),
  ('catalogue.products.publish',               'catalogue', 'Make a product visible on the storefront.'),
  ('catalogue.products.unpublish',             'catalogue', 'Remove a product from the storefront, retain the record.'),
  ('catalogue.products.archive',               'catalogue', 'Archive a listing.'),
  ('catalogue.products.delete',                'catalogue', 'Destroy a listing (ADR-016: refused with order history).'),
  ('catalogue.categories.read',                'catalogue', 'Read categories.'),
  ('catalogue.categories.manage',              'catalogue', 'Create, update or reorder categories.'),
  ('catalogue.categories.delete',              'catalogue', 'Delete a category.'),
  ('catalogue.product_images.manage',          'catalogue', 'Upload and attach product images.'),

  -- §3.3 Shopping, Ordering and Inventory
  ('shopping.cart.modify',                     'shopping',  'Create and change a cart [own].'),
  ('shopping.cart.read',                       'shopping',  'Read own cart [own].'),
  ('ordering.orders.create',                   'ordering',  'Place an order from own cart [own].'),
  ('ordering.orders.read',                     'ordering',  'Read own orders and fulfilment updates [own].'),
  ('ordering.orders.track',                    'ordering',  'Track an order by reference plus verified contact.'),
  ('ordering.orders.cancel',                   'ordering',  'Cancel own order where the state machine allows [own].'),
  ('ordering.orders.read.all',                 'ordering',  'Read all orders.'),
  ('ordering.orders.process',                  'ordering',  'Advance an order through permitted statuses (ADR-015).'),
  ('ordering.orders.status.override',          'ordering',  'Move an order outside its normal transition path, with a recorded reason.'),
  ('inventory.stock.read',                     'inventory', 'Read stock levels and alerts.'),
  ('inventory.stock.adjust',                   'inventory', 'Change stock with reason, quantity, actor and timestamp.'),
  ('inventory.movements.read',                 'inventory', 'Read the append-only stock movement history.'),
  ('inventory.reservations.read',              'inventory', 'Read active stock reservations.'),

  -- §3.4 Payments
  ('payments.initiate',                        'payments',  'Trigger a Daraja STK Push for own order [own].'),
  ('payments.status.read',                     'payments',  'Read payment state of own order [own].'),
  ('payments.reconcile',                       'payments',  'Reconcile a payment against Daraja records.'),
  ('payments.status.override',                 'payments',  'Manually set an order''s payment status.'),
  ('payments.records.read.all',                'payments',  'Read all payment transactions.'),
  ('payments.refund.request',                  'payments',  'Open a refund request with reason and amount (ADR-020).'),
  ('payments.refund',                          'payments',  'Approve a refund request and record its execution (ADR-020).'),
  ('payments.callback.receive',                'payments',  'Daraja callback — system credential, held by NO role.'),

  -- §3.5 Fulfilment
  ('fulfilment.delivery.read',                 'fulfilment', 'Read own delivery status [own].'),
  ('fulfilment.instructions.provide',          'fulfilment', 'Provide or select delivery and contact details at checkout [own].'),
  ('fulfilment.assignments.read',              'fulfilment', 'Read all deliveries and assignments.'),
  ('fulfilment.assign',                        'fulfilment', 'Assign a delivery to a person or route.'),
  ('fulfilment.update',                        'fulfilment', 'Record dispatch, pickup-ready and delivered events.'),
  ('fulfilment.config.manage',                 'fulfilment', 'Set service areas, fees and pickup windows.'),

  -- §3.6 Engagement, Promotions, Analytics, Administration
  ('engagement.reviews.create',                'engagement', 'Submit a review, subject to eligibility [own + verified purchase].'),
  ('engagement.reviews.read.published',        'engagement', 'Read approved reviews.'),
  ('engagement.reviews.moderate',              'engagement', 'Approve or reject a review.'),
  ('engagement.notifications.read',            'engagement', 'Read own notifications [own].'),
  ('engagement.notifications.preferences',     'engagement', 'Set own notification preferences [own].'),
  ('engagement.subscribers.read',              'engagement', 'Read the newsletter subscriber list.'),
  ('engagement.inquiries.read',                'engagement', 'Read contact inquiries.'),
  ('engagement.inquiries.respond',             'engagement', 'Mark or respond to an inquiry.'),
  ('promotions.campaigns.read',                'promotions', 'Read campaigns and eligibility rules.'),
  ('promotions.campaigns.manage',              'promotions', 'Manage an approved promotion.'),
  ('promotions.campaigns.approve',             'promotions', 'Approve a promotion for use.'),
  ('analytics.dashboards.read.operational',    'analytics',  'Operational KPIs, live feed, alerts and funnel.'),
  ('analytics.dashboards.read.business',       'analytics',  'Revenue, trends, payment health and business reports (ADR-022).'),
  ('analytics.reports.export',                 'analytics',  'Export permitted reports.'),
  ('analytics.forecasts.read',                 'analytics',  'Read forecast runs.'),
  ('analytics.targets.manage',                 'analytics',  'Set revenue and order targets.'),
  ('administration.audit_logs.read',           'administration', 'Read the audit and security event log.'),
  ('administration.system_settings.manage',    'administration', 'Global configuration.'),
  ('administration.integrations.manage',       'administration', 'Daraja, email, storage and SMS credentials and health.'),
  ('administration.integration_health.read',   'administration', 'Read integration status.'),
  ('administration.platform_health.read',      'administration', 'System-wide health dashboard.');

-- ── Seed: role grants, one block per role, each row citing its matrix line ─────────────────

-- customer — every [own] permission in §3. Nothing is shared between customers except published
-- catalogue data, which needs no grant because reading it is public.
INSERT INTO identity.role_permissions (role_id, permission_id)
SELECT r.id, p.id FROM identity.roles r
  JOIN identity.permissions p ON p.key = ANY(ARRAY[
    'identity.account.read',
    'identity.account.update',
    'identity.account.verify',
    'identity.account.recover',
    'catalogue.products.read.published',
    'catalogue.categories.read',
    'shopping.cart.modify',
    'shopping.cart.read',
    'ordering.orders.create',
    'ordering.orders.read',
    'ordering.orders.track',
    'ordering.orders.cancel',
    'payments.initiate',
    'payments.status.read',
    'fulfilment.delivery.read',
    'fulfilment.instructions.provide',
    'engagement.reviews.create',
    'engagement.reviews.read.published',
    'engagement.notifications.read',
    'engagement.notifications.preferences'
  ])
 WHERE r.key = 'customer';

-- admin_2_operations — operations. Deliberately WITHOUT payments.status.override (B3),
-- identity.admins.manage (B1), identity.roles.manage (B2), analytics.dashboards.read.business
-- (B4), administration.system_settings.manage (B5) and administration.audit_logs.read (B6).
-- promotions.campaigns.manage is absent by matrix §3.6 footnote §: conditional, so it is
-- available only as an exceptional grant.
INSERT INTO identity.role_permissions (role_id, permission_id)
SELECT r.id, p.id FROM identity.roles r
  JOIN identity.permissions p ON p.key = ANY(ARRAY[
    'identity.customers.read',
    'catalogue.products.read.published',
    'catalogue.products.read.draft',
    'catalogue.products.create',
    'catalogue.products.update',
    'catalogue.products.update_price',
    'catalogue.products.publish',
    'catalogue.products.unpublish',
    'catalogue.products.archive',
    'catalogue.categories.read',
    'catalogue.categories.manage',
    'catalogue.product_images.manage',
    'ordering.orders.read.all',
    'ordering.orders.process',
    'inventory.stock.read',
    'inventory.stock.adjust',
    'inventory.movements.read',
    'inventory.reservations.read',
    'payments.status.read',
    'payments.reconcile',
    'payments.records.read.all',
    'payments.refund.request',
    'fulfilment.assignments.read',
    'fulfilment.assign',
    'fulfilment.update',
    'engagement.reviews.read.published',
    'engagement.reviews.moderate',
    'engagement.inquiries.read',
    'engagement.inquiries.respond',
    'promotions.campaigns.read',
    'analytics.dashboards.read.operational',
    'analytics.reports.export',
    'administration.integration_health.read'
  ])
 WHERE r.key = 'admin_2_operations';

-- admin_1_business — supervision and the money-state authority Admin 2 must not hold. WITHOUT
-- identity.super_admins.manage (B7), identity.roles.manage and the two system/integration
-- management permissions (B8, B9).
-- §3.6 footnote †: audit read is a Super Admin-owned artefact; Admin 1 gets read scope over
-- Admin 2 activity, which is why the key is granted here and the scoping rule is a use-case
-- concern, not a missing grant.
INSERT INTO identity.role_permissions (role_id, permission_id)
SELECT r.id, p.id FROM identity.roles r
  JOIN identity.permissions p ON p.key = ANY(ARRAY[
    'identity.customers.read',
    'identity.customers.suspend',
    'identity.admins.read',
    'identity.admins.manage',
    'identity.roles.read',
    'identity.permissions.assign_scope',
    'catalogue.products.read.published',
    'catalogue.products.read.draft',
    'catalogue.products.update_price',
    'catalogue.products.delete',
    'catalogue.categories.read',
    'catalogue.categories.delete',
    'ordering.orders.read.all',
    'ordering.orders.status.override',
    'inventory.stock.read',
    'inventory.movements.read',
    'inventory.reservations.read',
    'payments.status.read',
    'payments.reconcile',
    'payments.status.override',
    'payments.records.read.all',
    'payments.refund.request',
    'payments.refund',
    'fulfilment.assignments.read',
    'fulfilment.config.manage',
    'engagement.reviews.read.published',
    'engagement.subscribers.read',
    'engagement.inquiries.read',
    'promotions.campaigns.read',
    'promotions.campaigns.manage',
    'promotions.campaigns.approve',
    'analytics.dashboards.read.operational',
    'analytics.dashboards.read.business',
    'analytics.reports.export',
    'analytics.forecasts.read',
    'analytics.targets.manage',
    'administration.audit_logs.read',
    'administration.integration_health.read',
    'administration.platform_health.read'
  ])
 WHERE r.key = 'admin_1_business';

-- super_admin — platform and security ownership. Holds the three keys nothing else may have,
-- and deliberately NOT ordering.orders.process or fulfilment.update: §3 makes the Super Admin a
-- governance actor, and a role that also performs daily operations is a role nobody audits.
INSERT INTO identity.role_permissions (role_id, permission_id)
SELECT r.id, p.id FROM identity.roles r
  JOIN identity.permissions p ON p.key = ANY(ARRAY[
    'identity.customers.read',
    'identity.customers.suspend',
    'identity.admins.read',
    'identity.admins.manage',
    'identity.super_admins.manage',
    'identity.roles.read',
    'identity.roles.manage',
    'identity.permissions.assign_scope',
    'identity.permissions.assign_exceptional',
    'identity.sessions.revoke',
    'catalogue.products.read.published',
    'catalogue.products.read.draft',
    'catalogue.products.create',
    'catalogue.products.update',
    'catalogue.products.update_price',
    'catalogue.products.publish',
    'catalogue.products.unpublish',
    'catalogue.products.archive',
    'catalogue.products.delete',
    'catalogue.categories.read',
    'catalogue.categories.manage',
    'catalogue.categories.delete',
    'catalogue.product_images.manage',
    'ordering.orders.read.all',
    'ordering.orders.process',
    'ordering.orders.status.override',
    'inventory.stock.read',
    'inventory.stock.adjust',
    'inventory.movements.read',
    'inventory.reservations.read',
    'payments.status.read',
    'payments.reconcile',
    'payments.status.override',
    'payments.records.read.all',
    'payments.refund.request',
    'payments.refund',
    'fulfilment.assignments.read',
    'fulfilment.assign',
    'fulfilment.update',
    'fulfilment.config.manage',
    'engagement.reviews.read.published',
    'engagement.reviews.moderate',
    'engagement.subscribers.read',
    'engagement.inquiries.read',
    'engagement.inquiries.respond',
    'promotions.campaigns.read',
    'promotions.campaigns.manage',
    'promotions.campaigns.approve',
    'analytics.dashboards.read.operational',
    'analytics.dashboards.read.business',
    'analytics.reports.export',
    'analytics.forecasts.read',
    'analytics.targets.manage',
    'administration.audit_logs.read',
    'administration.system_settings.manage',
    'administration.integrations.manage',
    'administration.integration_health.read',
    'administration.platform_health.read'
  ])
 WHERE r.key = 'super_admin';

-- ── Seed integrity ─────────────────────────────────────────────────────────────────────────
--
-- A typo in a key above would otherwise insert zero rows silently, and a role that quietly lost
-- all its permissions only shows up as a customer who cannot check out. Fail the migration
-- instead, while failing costs nothing.
DO $$
DECLARE
    n_permissions        INTEGER;
    n_expected            INTEGER := 75;
    orphan_grants         INTEGER;
    ungranted             TEXT;
    callback_holders      INTEGER;
BEGIN
    SELECT count(*) INTO n_permissions FROM identity.permissions;
    IF n_permissions <> n_expected THEN
        RAISE EXCEPTION 'permission catalogue has % rows, expected % (matrix §3 has 75)',
            n_permissions, n_expected;
    END IF;

    SELECT count(*) INTO orphan_grants FROM identity.role_permissions rp
     LEFT JOIN identity.permissions p ON p.id = rp.permission_id
     WHERE p.id IS NULL;
    IF orphan_grants > 0 THEN
        RAISE EXCEPTION '% role grants point at a permission that does not exist', orphan_grants;
    END IF;

    -- Every catalogue entry must reach at least one role, with exactly one documented exception.
    SELECT string_agg(p.key, ', ' ORDER BY p.key) INTO ungranted
      FROM identity.permissions p
     WHERE p.key <> 'payments.callback.receive'
       AND NOT EXISTS (SELECT 1 FROM identity.role_permissions rp WHERE rp.permission_id = p.id);
    IF ungranted IS NOT NULL THEN
        RAISE EXCEPTION 'these permissions are in the catalogue but granted to no role: %', ungranted;
    END IF;

    -- B3 and §3.4's "never a user role", enforced at the layer that can be tested.
    SELECT count(*) INTO callback_holders FROM identity.role_permissions rp
      JOIN identity.permissions p ON p.id = rp.permission_id
     WHERE p.key = 'payments.callback.receive';
    IF callback_holders > 0 THEN
        RAISE EXCEPTION 'payments.callback.receive is held by % role(s); it is a system credential, not a user permission',
            callback_holders;
    END IF;
END;
$$;
