const { pgPool } = require('./db');

const initPgTables = async () => {
  if (!process.env.PG_HOST || !process.env.PG_DATABASE) {
    console.warn('⚠️ PostgreSQL configuration missing, skipping table initialization.');
    return;
  }

  const client = await pgPool.connect();
  try {
    console.log('⏳ Initializing PostgreSQL tables...');
    await client.query(`
      CREATE TABLE IF NOT EXISTS orders (
        id SERIAL PRIMARY KEY,
        order_ref VARCHAR(50) UNIQUE NOT NULL,
        customer_name VARCHAR(255) NOT NULL,
        customer_phone VARCHAR(50),
        customer_email VARCHAR(255),
        total_amount DECIMAL(10, 2) NOT NULL,
        status VARCHAR(50) DEFAULT 'pending',
        payment_method VARCHAR(50) DEFAULT 'mpesa',
        payment_status VARCHAR(50) DEFAULT 'pending',
        mpesa_till_number VARCHAR(50),
        mpesa_transaction_id VARCHAR(100),
        mpesa_checkout_request_id VARCHAR(100),
        shipping_address TEXT,
        notes TEXT,
        items JSONB,
        created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
      );
      ALTER TABLE orders ADD COLUMN IF NOT EXISTS mpesa_checkout_request_id VARCHAR(100);
    `);

    await client.query(`
      CREATE TABLE IF NOT EXISTS subscribers (
        id SERIAL PRIMARY KEY,
        name VARCHAR(255),
        email VARCHAR(255) UNIQUE NOT NULL,
        is_active BOOLEAN DEFAULT true,
        subscribed_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
      );
    `);

    await client.query(`
      CREATE TABLE IF NOT EXISTS messages (
        id SERIAL PRIMARY KEY,
        name VARCHAR(255) NOT NULL,
        phone VARCHAR(50),
        email VARCHAR(255),
        message TEXT NOT NULL,
        is_read BOOLEAN DEFAULT false,
        created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
      );
    `);

    await client.query(`
      CREATE TABLE IF NOT EXISTS admins (
        id SERIAL PRIMARY KEY,
        name VARCHAR(255) NOT NULL,
        email VARCHAR(255) UNIQUE NOT NULL,
        password VARCHAR(255) NOT NULL,
        created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
      );
    `);

    // Analytics & Business Intelligence Tables
    await client.query(`
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
        period VARCHAR(20) NOT NULL,
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

      CREATE INDEX IF NOT EXISTS idx_analytics_snapshot_date ON analytics_daily_snapshot(date);
      CREATE INDEX IF NOT EXISTS idx_analytics_alerts_created ON analytics_alerts(created_at DESC);
    `);

    // Insert default managerial thresholds if none exist
    await client.query(`
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
    `);

    // Seed default admin if none exist or ensure primary admin credentials match
    const bcrypt = require('bcryptjs');
    const adminEmail = process.env.ADMIN_EMAIL || 'naojaventures@gmail.com';
    const adminPass = process.env.ADMIN_PASSWORD || 'naoja@1540';
    const { rows: existingAdmin } = await client.query('SELECT id FROM admins WHERE email = $1', [adminEmail]);
    const salt = await bcrypt.genSalt(10);
    const hashedPassword = await bcrypt.hash(adminPass, salt);

    if (existingAdmin.length === 0) {
      await client.query(
        'INSERT INTO admins (name, email, password) VALUES ($1, $2, $3)',
        ['Super Admin', adminEmail, hashedPassword]
      );
      console.log(`🔑 Admin created: ${adminEmail}`);
    } else {
      await client.query(
        'UPDATE admins SET password = $1 WHERE email = $2',
        [hashedPassword, adminEmail]
      );
      console.log(`🔑 Admin updated: ${adminEmail}`);
    }

    console.log('✅ PostgreSQL tables initialized successfully.');
  } catch (err) {
    console.error('❌ Error initializing PostgreSQL tables', err);
  } finally {
    client.release();
  }
};

module.exports = { initPgTables };
