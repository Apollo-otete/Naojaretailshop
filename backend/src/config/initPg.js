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
        shipping_address TEXT,
        notes TEXT,
        items JSONB,
        created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
      );
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

    // Seed default admin if none exist
    const { rows: admins } = await client.query('SELECT id FROM admins LIMIT 1');
    if (admins.length === 0) {
      const bcrypt = require('bcryptjs');
      const salt = await bcrypt.genSalt(10);
      const hashedPassword = await bcrypt.hash('admin123', salt);
      await client.query(
        'INSERT INTO admins (name, email, password) VALUES ($1, $2, $3)',
        ['Super Admin', 'admin@naojaventures.com', hashedPassword]
      );
      console.log('🔑 Default admin created: admin@naojaventures.com / admin123');
    }

    console.log('✅ PostgreSQL tables initialized successfully.');
  } catch (err) {
    console.error('❌ Error initializing PostgreSQL tables', err);
  } finally {
    client.release();
  }
};

module.exports = { initPgTables };
