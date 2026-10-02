-- ==========================================================
-- NAOJA VENTURES - ADVANCED BUSINESS INTELLIGENCE & ANALYTICS
-- Migration: Analytics Snapshots, Alerts, Targets & Velocity
-- ==========================================================

CREATE TABLE IF NOT EXISTS analytics_daily_snapshot (
    id SERIAL PRIMARY KEY,
    date DATE UNIQUE NOT NULL,
    total_revenue NUMERIC(14,2) DEFAULT 0,
    total_orders INTEGER DEFAULT 0,
    aov NUMERIC(10,2) DEFAULT 0,
    total_units INTEGER DEFAULT 0,
    new_customers INTEGER DEFAULT 0,
    failed_payments INTEGER DEFAULT 0,
    top_category VARCHAR(100),
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
);

CREATE TABLE IF NOT EXISTS analytics_alerts (
    id SERIAL PRIMARY KEY,
    type VARCHAR(50) NOT NULL,
    severity VARCHAR(20) NOT NULL DEFAULT 'warning',
    entity_type VARCHAR(50),
    entity_id VARCHAR(100),
    title VARCHAR(255) NOT NULL,
    message TEXT NOT NULL,
    is_acknowledged BOOLEAN DEFAULT FALSE,
    acknowledged_at TIMESTAMP,
    acknowledged_by VARCHAR(255),
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
);

CREATE TABLE IF NOT EXISTS analytics_targets (
    id SERIAL PRIMARY KEY,
    period VARCHAR(20) NOT NULL, -- 'week', 'month', 'year'
    period_start DATE NOT NULL,
    revenue_target NUMERIC(14,2) NOT NULL DEFAULT 0,
    orders_target INTEGER NOT NULL DEFAULT 0,
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    CONSTRAINT unique_period_target UNIQUE (period, period_start)
);

CREATE TABLE IF NOT EXISTS analytics_thresholds (
    id SERIAL PRIMARY KEY,
    key VARCHAR(100) UNIQUE NOT NULL,
    value JSONB NOT NULL,
    updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
);

CREATE TABLE IF NOT EXISTS product_sales_velocity (
    id SERIAL PRIMARY KEY,
    product_id VARCHAR(100) UNIQUE NOT NULL,
    product_name VARCHAR(255),
    daily_units_7d NUMERIC(8,2) DEFAULT 0,
    daily_units_30d NUMERIC(8,2) DEFAULT 0,
    daily_units_90d NUMERIC(8,2) DEFAULT 0,
    current_stock INTEGER DEFAULT 0,
    days_remaining NUMERIC(8,1) DEFAULT 999,
    last_calculated TIMESTAMP DEFAULT CURRENT_TIMESTAMP
);

-- Insert default managerial thresholds if none exist
INSERT INTO analytics_thresholds (key, value) VALUES
('default_thresholds', '{
    "low_stock_limit": 5,
    "critical_stock_limit": 2,
    "dead_stock_days": 30,
    "revenue_drop_pct": 20,
    "failed_payment_spike_count": 3,
    "stuck_order_minutes": 30,
    "fulfillment_delay_hours": 48,
    "target_revenue_weekly": 150000,
    "target_revenue_monthly": 600000,
    "target_revenue_yearly": 7200000
}'::jsonb)
ON CONFLICT (key) DO NOTHING;

-- Indices for performance
CREATE INDEX IF NOT EXISTS idx_analytics_snapshot_date ON analytics_daily_snapshot(date);
CREATE INDEX IF NOT EXISTS idx_analytics_alerts_created ON analytics_alerts(created_at DESC);
CREATE INDEX IF NOT EXISTS idx_analytics_alerts_ack ON analytics_alerts(is_acknowledged);
