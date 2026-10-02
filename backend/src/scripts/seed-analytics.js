// ==========================================================
// Seed Realistic 90-Day Analytics Data for Naoja Retail Shop
// Generates realistic orders, customers, and alerts
// ==========================================================

require('dotenv').config();
const { pgPool } = require('../config/db');

const KENYAN_NAMES = [
  'Kelvin Mwangi', 'Faith Wambui', 'Brian Ochieng', 'Mercy Chebet', 
  'Emmanuel Kiprop', 'Cynthia Achieng', 'Jackson Otieno', 'Diana Mutua',
  'Kennedy Kamau', 'Brenda Cherono', 'Dennis Barasa', 'Alice Wanjiku',
  'Samuel Mutiso', 'Sharon Jepkoech', 'Victor Wafula', 'Grace Nyambura'
];

const POPULAR_PRODUCTS = [
  { id: 1, name: 'Samsung Galaxy A15 (128GB/4GB)', category_id: 1, category_name: 'Mobile Phones', price: 18500 },
  { id: 2, name: 'Redmi 13C 128GB', category_id: 1, category_name: 'Mobile Phones', price: 14500 },
  { id: 3, name: 'Monocrystalline Solar Panel 200W', category_id: 8, category_name: 'Renewable Energy', price: 12500 },
  { id: 4, name: 'Solar Gel Battery 100Ah 12V', category_id: 8, category_name: 'Renewable Energy', price: 19800 },
  { id: 5, name: 'Oraimo 65W Fast Charger (GaN)', category_id: 3, category_name: 'Phone Accessories', price: 2800 },
  { id: 6, name: 'JBL Charge 5 Portable Speaker', category_id: 4, category_name: 'Audio Systems', price: 16500 },
  { id: 7, name: 'HP EliteBook 840 G5 Core i5', category_id: 2, category_name: 'Computers & Laptops', price: 34000 },
  { id: 8, name: 'Vitron 32 Inch Frameless Smart TV', category_id: 6, category_name: 'TVs & Entertainment', price: 13500 },
  { id: 9, name: 'Type-C Braided Fast Cable 2M', category_id: 3, category_name: 'Phone Accessories', price: 650 },
  { id: 10, name: 'Solar Hybrid Inverter 1.5kVA', category_id: 8, category_name: 'Renewable Energy', price: 28000 }
];

async function seedAnalyticsData() {
  console.log('🚀 Seeding 90 days of realistic orders and analytics...');

  const client = await pgPool.connect();

  try {
    // Check if orders already populated
    const checkRes = await client.query('SELECT COUNT(*) as count FROM orders');
    const existingCount = Number(checkRes.rows[0].count);

    if (existingCount >= 50) {
      console.log(`ℹ️ Database already has ${existingCount} orders. Skipping bulk generation.`);
    } else {
      console.log('Generating 90 days of synthetic Kenyan retail transactions...');

      const now = new Date();
      let totalCreated = 0;

      // Loop through 90 days backwards
      for (let dayOffset = 90; dayOffset >= 0; dayOffset--) {
        const orderDate = new Date(now);
        orderDate.setDate(now.getDate() - dayOffset);

        // Day of week seasonality: Friday & Sunday higher volume
        const dayOfWeek = orderDate.getDay();
        const baseOrders = (dayOfWeek === 5 || dayOfWeek === 0) ? 3 : 2;
        const dailyOrdersCount = baseOrders + Math.floor(Math.random() * 3);

        for (let i = 0; i < dailyOrdersCount; i++) {
          const orderTime = new Date(orderDate);
          orderTime.setHours(8 + Math.floor(Math.random() * 12), Math.floor(Math.random() * 60));

          const customer = KENYAN_NAMES[Math.floor(Math.random() * KENYAN_NAMES.length)];
          const phone = '254' + (700000000 + Math.floor(Math.random() * 99999999));
          const email = customer.toLowerCase().replace(/ /g, '.') + '@gmail.com';
          const orderRef = 'NJ-' + Math.floor(100000 + Math.random() * 900000);

          // Select 1 to 3 items
          const itemsCount = 1 + (Math.random() > 0.7 ? 1 : 0);
          const orderItems = [];
          let totalAmount = 0;

          for (let j = 0; j < itemsCount; j++) {
            const prod = POPULAR_PRODUCTS[Math.floor(Math.random() * POPULAR_PRODUCTS.length)];
            const qty = (prod.price < 3000 && Math.random() > 0.5) ? 2 : 1;
            orderItems.push({
              product_id: prod.id,
              name: prod.name,
              category_id: prod.category_id,
              category_name: prod.category_name,
              unit_price: prod.price,
              quantity: qty
            });
            totalAmount += prod.price * qty;
          }

          // 92% completed, 5% failed, 3% pending
          const roll = Math.random();
          let paymentStatus = 'completed';
          let orderStatus = 'delivered';
          if (roll > 0.95) {
            paymentStatus = 'failed';
            orderStatus = 'cancelled';
          } else if (roll > 0.92) {
            paymentStatus = 'pending';
            orderStatus = 'pending';
          } else {
            // Completed
            if (dayOffset <= 1) {
              orderStatus = Math.random() > 0.5 ? 'confirmed' : 'processing';
            } else if (dayOffset <= 3) {
              orderStatus = 'shipped';
            } else {
              orderStatus = 'delivered';
            }
          }

          const mpesaReceipt = paymentStatus === 'completed' ? 'QK' + Math.floor(10000000 + Math.random() * 90000000) : null;
          const address = Math.random() > 0.4 ? 'Store Pickup (Lurambi Shop)' : 'Delivery to Kakamega Town';

          await client.query(`
            INSERT INTO orders (
              order_ref, customer_name, customer_phone, customer_email,
              total_amount, status, payment_method, payment_status,
              mpesa_till_number, mpesa_transaction_id, shipping_address,
              items, created_at
            ) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13)
          `, [
            orderRef, customer, phone, email,
            totalAmount, orderStatus, 'mpesa', paymentStatus,
            '4149288', mpesaReceipt, address,
            JSON.stringify(orderItems), orderTime
          ]);

          totalCreated++;
        }
      }

      console.log(`✅ Inserted ${totalCreated} synthetic historical orders.`);
    }

    // Seed sample managerial alerts
    await client.query(`
      INSERT INTO analytics_alerts (type, severity, title, message, entity_type, entity_id) VALUES
      ('low_stock', 'warning', 'Low Stock Alert: Oraimo 65W Fast Charger', 'Stock count is 3 units remaining. Current velocity suggests stock-out in 4 days.', 'product', '5'),
      ('low_stock', 'critical', 'Critical Stock: Monocrystalline Solar Panel 200W', 'Only 1 unit in warehouse. Reorder suggested: 10 units.', 'product', '3'),
      ('payment_failure_spike', 'info', 'Daraja Notification', 'Payment gateway operated with 95.8% success rate today.', 'payment', 'daraja')
      ON CONFLICT DO NOTHING;
    `);

    // Seed targets
    const currentMonthStart = new Date(new Date().getFullYear(), new Date().getMonth(), 1).toISOString().split('T')[0];
    await client.query(`
      INSERT INTO analytics_targets (period, period_start, revenue_target, orders_target)
      VALUES ('month', $1, 650000, 160)
      ON CONFLICT (period, period_start) DO NOTHING;
    `, [currentMonthStart]);

    console.log('✅ Analytics targets and alerts seeded successfully.');
  } catch (err) {
    console.error('Error seeding analytics:', err.message);
  } finally {
    client.release();
    process.exit(0);
  }
}

seedAnalyticsData();
