require('dotenv').config();
const mongoose = require('mongoose');
const { pgPool } = require('../config/db');
const { initPgTables } = require('../config/initPg');
const Product = require('../models/Product');
const Category = require('../models/Category');

const DEFAULT_CATEGORIES = [
  { id: 1, name: 'Mobile Phones', slug: 'mobile-phones', icon: '📱', product_count: 3 },
  { id: 2, name: 'Audio Devices', slug: 'audio-devices', icon: '🎧', product_count: 2 },
  { id: 3, name: 'Television Products', slug: 'television-products', icon: '📺', product_count: 1 }
];

const DEFAULT_PRODUCTS = [
  {
    id: 1,
    name: 'iPhone 15 Pro Max 256GB',
    slug: 'iphone-15-pro-max-256gb',
    description: 'Apple iPhone 15 Pro Max featuring a titanium design, the groundbreaking A17 Pro chip, a customizable Action button, and the most powerful iPhone camera system ever. Brand new in box with 1 year local warranty.',
    price: 160000.00,
    stock_quantity: 15,
    category_id: 1,
    subcategory: 'Smartphones',
    images: ['https://images.unsplash.com/photo-1695048133142-1a20484d2569?w=600&auto=format&fit=crop&q=80'],
    status: 'in_stock',
    is_featured: true,
    views: 412,
    rating_avg: 4.8,
    rating_count: 10
  }
];

const seedDB = async () => {
  try {
    // Mongo Seed
    if (process.env.MONGODB_URI) {
      await mongoose.connect(process.env.MONGODB_URI);
      console.log('Connected to MongoDB for seeding...');
      
      await Category.deleteMany({});
      await Product.deleteMany({});
      
      await Category.insertMany(DEFAULT_CATEGORIES);
      await Product.insertMany(DEFAULT_PRODUCTS);
      console.log('✅ MongoDB seeded');
      await mongoose.disconnect();
    }
    
    // PG Seed
    if (process.env.PG_HOST && process.env.PG_DATABASE) {
      await initPgTables();
      const client = await pgPool.connect();
      // Insert a test order
      await client.query(`
        INSERT INTO orders (order_ref, customer_name, total_amount, payment_status, items)
        VALUES ('NJ-2026-0001', 'Test User', 160000, 'paid', '[{"product_id": 1, "quantity": 1, "total_price": 160000}]')
        ON CONFLICT (order_ref) DO NOTHING;
      `);
      console.log('✅ PostgreSQL seeded');
      client.release();
      await pgPool.end();
    }
    
    console.log('🎉 Seeding complete!');
    process.exit(0);
  } catch (err) {
    console.error('❌ Seeding failed:', err);
    process.exit(1);
  }
};

seedDB();
