-- 0006: Shopping context — carts and cart_items (§5.3).
--
-- Source of truth: docs/phase-1-domain-model-erd.md §5.3. A guest cart is keyed by a hashed
-- session token; a user cart by user_id. Cart item prices are snapshots (add-time) plus a
-- current price recomputed on read by the service layer.

CREATE TABLE commerce.carts (
    id                  BIGSERIAL PRIMARY KEY,
    user_id             BIGINT REFERENCES identity.users (id) ON DELETE CASCADE,
    session_token_hash  TEXT,
    created_at          TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at          TIMESTAMPTZ NOT NULL DEFAULT now(),
    merged_into_cart_id BIGINT REFERENCES commerce.carts (id) ON DELETE SET NULL,
    CHECK (user_id IS NOT NULL OR session_token_hash IS NOT NULL)
);

CREATE INDEX ON commerce.carts (user_id) WHERE user_id IS NOT NULL;
CREATE UNIQUE INDEX ON commerce.carts (session_token_hash) WHERE session_token_hash IS NOT NULL;

CREATE TRIGGER carts_touch_updated_at
    BEFORE UPDATE ON commerce.carts
    FOR EACH ROW EXECUTE FUNCTION identity.touch_updated_at();

CREATE TABLE commerce.cart_items (
    cart_id              BIGINT NOT NULL REFERENCES commerce.carts (id) ON DELETE CASCADE,
    product_id           BIGINT NOT NULL REFERENCES catalogue.products (id),
    quantity             INTEGER NOT NULL CHECK (quantity > 0 AND quantity <= 999),
    price_snapshot_cents BIGINT NOT NULL CHECK (price_snapshot_cents >= 0),
    added_at_price_cents BIGINT NOT NULL CHECK (added_at_price_cents >= 0),
    created_at           TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at           TIMESTAMPTZ NOT NULL DEFAULT now(),
    PRIMARY KEY (cart_id, product_id)
);

CREATE TRIGGER cart_items_touch_updated_at
    BEFORE UPDATE ON commerce.cart_items
    FOR EACH ROW EXECUTE FUNCTION identity.touch_updated_at();

