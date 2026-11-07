-- 0001: PostgreSQL schemas, one per bounded context (§7).
--
-- Schemas rather than table-name prefixes because §7's module layout maps a Go module onto a
-- schema: a repository in the Ordering module only ever touches commerce.*, and cross-context
-- joins become visible in code review instead of hiding inside one flat namespace.

CREATE EXTENSION IF NOT EXISTS citext;    -- case-insensitive emails, slugs, keys
CREATE EXTENSION IF NOT EXISTS pgcrypto;  -- digest/hash helpers used by tokens and receipts

CREATE SCHEMA identity;      -- users, roles, permissions, sessions, invitations
CREATE SCHEMA catalogue;     -- categories, products, product_images
CREATE SCHEMA commerce;      -- carts, cart_items, orders, order_lines, order_status_history
CREATE SCHEMA payments;      -- payments, payment_attempts, payment_transactions, refunds
CREATE SCHEMA inventory;     -- inventory, stock_movements, stock_reservations
CREATE SCHEMA fulfilment;    -- delivery_zones, deliveries, delivery_assignments, store_hours
CREATE SCHEMA engagement;    -- reviews, notifications, subscribers, inquiries
CREATE SCHEMA analytics;     -- events, daily_snapshots, sales_summaries, targets, thresholds
CREATE SCHEMA administration; -- audit_logs, system_settings, outbox_events

COMMENT ON SCHEMA identity IS 'Bounded context: Identity & Access';
COMMENT ON SCHEMA catalogue IS 'Bounded context: Catalogue';
COMMENT ON SCHEMA commerce IS 'Bounded context: Shopping + Ordering';
COMMENT ON SCHEMA payments IS 'Bounded context: Payments';
COMMENT ON SCHEMA inventory IS 'Bounded context: Inventory';
COMMENT ON SCHEMA fulfilment IS 'Bounded context: Fulfilment';
COMMENT ON SCHEMA engagement IS 'Bounded context: Customer Engagement';
COMMENT ON SCHEMA analytics IS 'Bounded context: Analytics & Intelligence';
COMMENT ON SCHEMA administration IS 'Bounded context: Administration & Audit';
