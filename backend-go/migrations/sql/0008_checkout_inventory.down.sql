-- 0008 down: remove P0 checkout/inventory/idempotency extensions.
-- Order matters: drop FKs/columns referencing checkout_quotes before dropping checkout_quotes.

DROP TABLE IF EXISTS commerce.order_idempotency_keys;
DROP TABLE IF EXISTS commerce.checkout_quote_lines;

DROP INDEX IF EXISTS commerce.orders_quote_id_idx;
DROP INDEX IF EXISTS commerce.orders_fulfilment_method_idx;

ALTER TABLE commerce.orders
    DROP COLUMN IF EXISTS fulfilment_method,
    DROP COLUMN IF EXISTS quote_id,
    DROP COLUMN IF EXISTS contact_name,
    DROP COLUMN IF EXISTS contact_email,
    DROP COLUMN IF EXISTS delivery_zone_id,
    DROP COLUMN IF EXISTS delivery_area,
    DROP COLUMN IF EXISTS delivery_county,
    DROP COLUMN IF EXISTS delivery_landmark,
    DROP COLUMN IF EXISTS pickup_window_id,
    DROP COLUMN IF EXISTS notes;

DROP TABLE IF EXISTS commerce.checkout_quotes;
DROP SEQUENCE IF EXISTS commerce.order_ref_seq;

DROP TABLE IF EXISTS inventory.stock_movements;
DROP TABLE IF EXISTS inventory.stock_reservations;
DROP TABLE IF EXISTS inventory.inventory;

-- Restore the pre-0008 status constraints (best-effort; legacy-compatible values retained).
ALTER TABLE commerce.orders DROP CONSTRAINT IF EXISTS orders_status_check;
ALTER TABLE commerce.orders ADD CONSTRAINT orders_status_check CHECK (status IN (
    'pending_payment','paid','processing','ready','dispatched','delivered','cancelled','failed','expired'
));

ALTER TABLE commerce.orders DROP CONSTRAINT IF EXISTS orders_payment_status_check;
ALTER TABLE commerce.orders ADD CONSTRAINT orders_payment_status_check CHECK (payment_status IN (
    'pending','paid','failed','refunded'
));

