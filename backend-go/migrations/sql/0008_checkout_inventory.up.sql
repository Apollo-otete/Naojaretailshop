-- 0008: P0 contract-safe checkout foundations — inventory, reservations, quotes, idempotency,
-- and order/placement extensions (T-01 anti-oversell, idempotent placement, §19 step 6).

-- ── Inventory (ERD 5.5) ─────────────────────────────────────────────────────
-- Separate stock truth from the product document: reserved <= on_hand is enforced at the row
-- level, and every on_hand change has a corresponding stock movement.

CREATE TABLE inventory.inventory (
    id                       BIGSERIAL PRIMARY KEY,
    product_id               BIGINT NOT NULL UNIQUE REFERENCES catalogue.products (id) ON DELETE CASCADE,
    on_hand                  INTEGER NOT NULL DEFAULT 0 CHECK (on_hand >= 0),
    reserved                 INTEGER NOT NULL DEFAULT 0 CHECK (reserved >= 0),
    available                INTEGER GENERATED ALWAYS AS (on_hand - reserved) STORED,
    track_stock              BOOLEAN NOT NULL DEFAULT TRUE,
    low_stock_threshold      INTEGER NOT NULL DEFAULT 5,
    critical_stock_threshold INTEGER NOT NULL DEFAULT 2,
    created_at               TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at               TIMESTAMPTZ NOT NULL DEFAULT now(),
    CHECK (reserved <= on_hand)
);

CREATE INDEX ON inventory.inventory (available);
CREATE INDEX ON inventory.inventory (product_id);

CREATE TRIGGER inventory_touch_updated_at
    BEFORE UPDATE ON inventory.inventory
    FOR EACH ROW EXECUTE FUNCTION identity.touch_updated_at();

-- Bootstrap inventory for any existing products so the system is operable before a full
-- stock-load/migration. Quantities are a pragmatic default and must be reconciled from the
-- authoritative source of truth (P3 migration/cutover).
INSERT INTO inventory.inventory (product_id, on_hand, reserved, track_stock, low_stock_threshold, critical_stock_threshold)
SELECT id,
       100,
       0,
       TRUE,
       COALESCE(low_stock_threshold, 5),
       COALESCE(critical_stock_threshold, 2)
FROM catalogue.products
ON CONFLICT (product_id) DO NOTHING;

-- One reservation per (order, product) makes release/consume idempotent by construction.
CREATE TABLE inventory.stock_reservations (
    id           BIGSERIAL PRIMARY KEY,
    order_id     BIGINT NOT NULL REFERENCES commerce.orders (id) ON DELETE CASCADE,
    product_id   BIGINT NOT NULL REFERENCES catalogue.products (id) ON DELETE RESTRICT,
    quantity     INTEGER NOT NULL CHECK (quantity > 0),
    status       TEXT NOT NULL DEFAULT 'active'
                 CHECK (status IN ('active', 'released', 'consumed')),
    expires_at   TIMESTAMPTZ,
    created_at   TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at   TIMESTAMPTZ NOT NULL DEFAULT now(),
    UNIQUE (order_id, product_id)
);

CREATE INDEX ON inventory.stock_reservations (status, expires_at);
CREATE INDEX ON inventory.stock_reservations (order_id);

CREATE TRIGGER stock_reservations_touch_updated_at
    BEFORE UPDATE ON inventory.stock_reservations
    FOR EACH ROW EXECUTE FUNCTION identity.touch_updated_at();

-- Append-only movement history. Corrections are compensating entries, never rewrites.
CREATE TABLE inventory.stock_movements (
    id               BIGSERIAL PRIMARY KEY,
    product_id       BIGINT NOT NULL REFERENCES catalogue.products (id) ON DELETE RESTRICT,
    order_id         BIGINT REFERENCES commerce.orders (id) ON DELETE SET NULL,
    reservation_id   BIGINT REFERENCES inventory.stock_reservations (id) ON DELETE SET NULL,
    delta_on_hand    INTEGER NOT NULL DEFAULT 0,
    delta_reserved   INTEGER NOT NULL DEFAULT 0,
    on_hand_after    INTEGER NOT NULL CHECK (on_hand_after >= 0),
    reserved_after   INTEGER NOT NULL CHECK (reserved_after >= 0),
    kind             TEXT NOT NULL
                     CHECK (kind IN ('bootstrap','sale','reservation','reservation_release','return','adjustment','consumption')),
    reason           TEXT,
    actor_id         BIGINT REFERENCES identity.users (id) ON DELETE SET NULL,
    occurred_at      TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX ON inventory.stock_movements (product_id, occurred_at DESC);
CREATE INDEX ON inventory.stock_movements (order_id);

-- ── Checkout quotes ─────────────────────────────────────────────────────────
CREATE TABLE commerce.checkout_quotes (
    id                   BIGSERIAL PRIMARY KEY,
    user_id              BIGINT REFERENCES identity.users (id) ON DELETE SET NULL,
    cart_id              BIGINT REFERENCES commerce.carts (id) ON DELETE SET NULL,
    session_token_hash   TEXT,
    fulfilment_method    TEXT NOT NULL CHECK (fulfilment_method IN ('pickup', 'delivery')),
    contact_name         TEXT NOT NULL,
    contact_phone        TEXT NOT NULL,
    contact_email        TEXT,
    delivery_zone_id     BIGINT,
    delivery_address_line TEXT,
    delivery_area        TEXT,
    delivery_county      TEXT,
    delivery_landmark    TEXT,
    pickup_window_id     BIGINT,
    notes                TEXT,
    subtotal_cents       BIGINT NOT NULL CHECK (subtotal_cents >= 0),
    delivery_fee_cents   BIGINT NOT NULL DEFAULT 0 CHECK (delivery_fee_cents >= 0),
    discount_cents       BIGINT NOT NULL DEFAULT 0 CHECK (discount_cents >= 0),
    total_cents          BIGINT NOT NULL CHECK (total_cents >= 0),
    currency             TEXT NOT NULL DEFAULT 'KES',
    pricing_hash         TEXT NOT NULL,
    pricing_changed      BOOLEAN NOT NULL DEFAULT FALSE,
    expires_at           TIMESTAMPTZ NOT NULL,
    created_at           TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX ON commerce.checkout_quotes (user_id, created_at DESC);
CREATE INDEX ON commerce.checkout_quotes (expires_at);

CREATE TABLE commerce.checkout_quote_lines (
    id               BIGSERIAL PRIMARY KEY,
    quote_id         BIGINT NOT NULL REFERENCES commerce.checkout_quotes (id) ON DELETE CASCADE,
    product_id       BIGINT NOT NULL REFERENCES catalogue.products (id) ON DELETE RESTRICT,
    name             TEXT NOT NULL,
    sku              TEXT NOT NULL,
    unit_price_cents BIGINT NOT NULL CHECK (unit_price_cents >= 0),
    quantity         INTEGER NOT NULL CHECK (quantity > 0),
    line_total_cents BIGINT NOT NULL CHECK (line_total_cents >= 0)
);

CREATE INDEX ON commerce.checkout_quote_lines (quote_id);

-- ── Idempotent order placement (T-01 / contract) ────────────────────────────
CREATE TABLE commerce.order_idempotency_keys (
    id              BIGSERIAL PRIMARY KEY,
    user_id         BIGINT NOT NULL REFERENCES identity.users (id) ON DELETE CASCADE,
    idem_key        TEXT NOT NULL,
    request_hash    TEXT NOT NULL,
    status_code     INTEGER NOT NULL,
    response_body   JSONB NOT NULL,
    order_reference TEXT,
    created_at      TIMESTAMPTZ NOT NULL DEFAULT now(),
    UNIQUE (user_id, idem_key)
);

CREATE INDEX ON commerce.order_idempotency_keys (created_at);

-- Reference sequence: NJ-<year>-<sequence>, never count(*)+1.
CREATE SEQUENCE IF NOT EXISTS commerce.order_ref_seq START 1;

-- ── Orders extensions for contract placement ────────────────────────────────
ALTER TABLE commerce.orders
    ADD COLUMN IF NOT EXISTS fulfilment_method TEXT,
    ADD COLUMN IF NOT EXISTS quote_id BIGINT REFERENCES commerce.checkout_quotes (id) ON DELETE SET NULL,
    ADD COLUMN IF NOT EXISTS contact_name TEXT,
    ADD COLUMN IF NOT EXISTS contact_email TEXT,
    ADD COLUMN IF NOT EXISTS delivery_zone_id BIGINT,
    ADD COLUMN IF NOT EXISTS delivery_area TEXT,
    ADD COLUMN IF NOT EXISTS delivery_county TEXT,
    ADD COLUMN IF NOT EXISTS delivery_landmark TEXT,
    ADD COLUMN IF NOT EXISTS pickup_window_id BIGINT,
    ADD COLUMN IF NOT EXISTS notes TEXT;

-- Align statuses with the contract while keeping legacy values accepted.
ALTER TABLE commerce.orders DROP CONSTRAINT IF EXISTS orders_status_check;
ALTER TABLE commerce.orders ADD CONSTRAINT orders_status_check CHECK (status IN (
    'pending', 'pending_payment', 'confirmed', 'paid', 'processing', 'ready',
    'dispatched', 'delivered', 'cancelled', 'failed', 'expired', 'returned', 'refunded'
));

ALTER TABLE commerce.orders DROP CONSTRAINT IF EXISTS orders_payment_status_check;
ALTER TABLE commerce.orders ADD CONSTRAINT orders_payment_status_check CHECK (payment_status IN (
    'pending', 'paid', 'failed', 'refunded', 'expired'
));

CREATE INDEX IF NOT EXISTS orders_quote_id_idx ON commerce.orders (quote_id);
CREATE INDEX IF NOT EXISTS orders_fulfilment_method_idx ON commerce.orders (fulfilment_method);
