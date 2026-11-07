-- 0007: Ordering context — orders, order_items, order_status_history (§5.3).

CREATE TABLE commerce.orders (
    id                BIGSERIAL PRIMARY KEY,
    reference         TEXT UNIQUE NOT NULL,
    user_id           BIGINT REFERENCES identity.users (id),
    status            TEXT NOT NULL DEFAULT 'pending_payment'
                      CHECK (status IN ('pending_payment','paid','processing','ready','dispatched','delivered','cancelled','failed','expired')),
    payment_status    TEXT NOT NULL DEFAULT 'pending'
                      CHECK (payment_status IN ('pending','paid','failed','refunded')),
    subtotal_cents    BIGINT NOT NULL CHECK (subtotal_cents >= 0),
    delivery_fee_cents BIGINT NOT NULL DEFAULT 0 CHECK (delivery_fee_cents >= 0),
    total_cents       BIGINT NOT NULL CHECK (total_cents >= 0),
    currency          TEXT NOT NULL DEFAULT 'KES',
    delivery_address  TEXT,
    contact_phone     TEXT,
    delivery_instructions TEXT,
    expires_at        TIMESTAMPTZ,
    created_at        TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at        TIMESTAMPTZ NOT NULL DEFAULT now(),
    paid_at           TIMESTAMPTZ,
    cancelled_at      TIMESTAMPTZ
);

CREATE INDEX ON commerce.orders (user_id);
CREATE INDEX ON commerce.orders (status);
CREATE INDEX ON commerce.orders (created_at DESC);

CREATE TRIGGER orders_touch_updated_at
    BEFORE UPDATE ON commerce.orders
    FOR EACH ROW EXECUTE FUNCTION identity.touch_updated_at();

CREATE TABLE commerce.order_items (
    id                BIGSERIAL PRIMARY KEY,
    order_id          BIGINT NOT NULL REFERENCES commerce.orders (id) ON DELETE CASCADE,
    product_id        BIGINT REFERENCES catalogue.products (id),
    product_name      TEXT NOT NULL,
    sku               TEXT NOT NULL,
    quantity          INTEGER NOT NULL CHECK (quantity > 0),
    unit_price_cents  BIGINT NOT NULL CHECK (unit_price_cents >= 0),
    line_total_cents  BIGINT NOT NULL CHECK (line_total_cents >= 0),
    created_at        TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX ON commerce.order_items (order_id);
CREATE INDEX ON commerce.order_items (product_id);

CREATE TABLE commerce.order_status_history (
    id          BIGSERIAL PRIMARY KEY,
    order_id    BIGINT NOT NULL REFERENCES commerce.orders (id) ON DELETE CASCADE,
    from_status TEXT,
    to_status   TEXT NOT NULL,
    reason      TEXT,
    actor_id    BIGINT REFERENCES identity.users (id),
    occurred_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX ON commerce.order_status_history (order_id, occurred_at DESC);

