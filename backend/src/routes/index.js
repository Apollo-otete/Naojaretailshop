const express = require('express');
const router = express.Router();
const path = require('path');
const multer = require('multer');
const Product = require('../models/Product');
const Category = require('../models/Category');
const Review = require('../models/Review');
const { pgPool } = require('../config/db');
const auth = require('../middleware/auth');

// Multer config — save files to backend/uploads/
const storage = multer.diskStorage({
  destination: (req, file, cb) => {
    cb(null, path.join(__dirname, '../../uploads'));
  },
  filename: (req, file, cb) => {
    const ext = path.extname(file.originalname);
    const unique = `${Date.now()}-${Math.round(Math.random() * 1e9)}${ext}`;
    cb(null, unique);
  },
});
const upload = multer({
  storage,
  limits: { fileSize: 10 * 1024 * 1024 }, // 10 MB max
  fileFilter: (req, file, cb) => {
    if (file.mimetype.startsWith('image/')) cb(null, true);
    else cb(new Error('Only image files are allowed'));
  },
});

// ========================
// PRODUCTS
// ========================
router.get('/products', async (req, res) => {
  try {
    const { category, search, admin } = req.query;
    let query = {};

    if (category) {
      const cat = await Category.findOne({ slug: category });
      if (cat) query.category_id = cat.id;
      else return res.json([]);
    }

    if (search) {
      query.$or = [
        { name: { $regex: search, $options: 'i' } },
        { description: { $regex: search, $options: 'i' } },
        { subcategory: { $regex: search, $options: 'i' } }
      ];
    }

    const products = await Product.find(query).sort({ id: 1 });
    res.json(products);
  } catch (error) {
    res.status(500).json({ message: error.message });
  }
});

router.get('/products/:slugOrId', async (req, res) => {
  try {
    const { slugOrId } = req.params;
    let query = { slug: slugOrId };
    if (!isNaN(slugOrId)) {
      query = { $or: [{ slug: slugOrId }, { id: parseInt(slugOrId) }] };
    }

    const product = await Product.findOneAndUpdate(
      query,
      { $inc: { views: 1 } },
      { new: true }
    );

    if (!product) return res.status(404).json({ message: 'Product not found' });
    res.json(product);
  } catch (error) {
    res.status(500).json({ message: error.message });
  }
});

router.post('/products', auth, async (req, res) => {
  try {
    const lastProduct = await Product.findOne().sort({ id: -1 });
    const newId = lastProduct ? lastProduct.id + 1 : 1;

    const product = new Product({
      id: newId,
      ...req.body,
      slug: req.body.slug || req.body.name.toLowerCase().replace(/[^a-z0-9]+/g, '-')
    });

    await product.save();
    res.status(201).json(product);
  } catch (error) {
    res.status(400).json({ message: error.message });
  }
});

router.put('/products/:id', auth, async (req, res) => {
  try {
    let updateData = { ...req.body };
    if (updateData.stock_quantity !== undefined) {
      updateData.status = updateData.stock_quantity <= 0 ? 'out_of_stock' : (updateData.stock_quantity <= 5 ? 'low_stock' : 'in_stock');
    }
    const product = await Product.findOneAndUpdate({ id: req.params.id }, updateData, { new: true });
    if (!product) return res.status(404).json({ message: 'Product not found' });
    res.json(product);
  } catch (error) {
    res.status(400).json({ message: error.message });
  }
});

router.delete('/products/:id', auth, async (req, res) => {
  try {
    await Product.findOneAndDelete({ id: req.params.id });
    res.json({ message: 'Product deleted successfully', id: req.params.id });
  } catch (error) {
    res.status(500).json({ message: error.message });
  }
});

// ========================
// CATEGORIES
// ========================
router.get('/categories', async (req, res) => {
  try {
    const categories = await Category.find().sort({ id: 1 });
    // Note: To match mock exact behavior, product_count can be dynamically calculated,
    // but we can also rely on it being populated in DB for performance.
    res.json(categories);
  } catch (error) {
    res.status(500).json({ message: error.message });
  }
});

router.post('/categories', auth, async (req, res) => {
  try {
    const lastCat = await Category.findOne().sort({ id: -1 });
    const newId = lastCat ? lastCat.id + 1 : 1;

    const category = new Category({
      id: newId,
      ...req.body,
      slug: req.body.slug || req.body.name.toLowerCase().replace(/[^a-z0-9]+/g, '-')
    });

    await category.save();
    res.status(201).json(category);
  } catch (error) {
    res.status(400).json({ message: error.message });
  }
});

router.put('/categories/:id', auth, async (req, res) => {
  try {
    const category = await Category.findOneAndUpdate({ id: req.params.id }, req.body, { new: true });
    if (!category) return res.status(404).json({ message: 'Category not found' });
    res.json(category);
  } catch (error) {
    res.status(400).json({ message: error.message });
  }
});

router.delete('/categories/:id', auth, async (req, res) => {
  try {
    await Category.findOneAndDelete({ id: req.params.id });
    res.json({ message: 'Category deleted successfully', id: req.params.id });
  } catch (error) {
    res.status(500).json({ message: error.message });
  }
});

// ========================
// ORDERS (PostgreSQL)
// ========================
router.get('/orders', async (req, res) => {
  try {
    const { rows } = await pgPool.query('SELECT * FROM orders ORDER BY created_at DESC');
    res.json(rows);
  } catch (error) {
    res.status(500).json({ message: error.message });
  }
});

router.post('/orders', async (req, res) => {
  try {
    const { customer_name, customer_phone, customer_email, total_amount, payment_method, mpesa_transaction_id, shipping_address, notes, items } = req.body;

    const { rows: countRows } = await pgPool.query('SELECT count(*) FROM orders');
    const orderCount = parseInt(countRows[0].count, 10);
    const order_ref = `NJ-${new Date().getFullYear()}-${String(orderCount + 1).padStart(4, '0')}`;

    const result = await pgPool.query(`
      INSERT INTO orders (
        order_ref, customer_name, customer_phone, customer_email, total_amount,
        payment_method, mpesa_transaction_id, shipping_address, notes, items
      ) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10) RETURNING *
    `, [order_ref, customer_name, customer_phone, customer_email, total_amount, payment_method, mpesa_transaction_id, shipping_address, notes, JSON.stringify(items)]);

    // Decrement stock in Mongo
    if (items && items.length > 0) {
      for (const item of items) {
        await Product.findOneAndUpdate(
          { id: item.product_id },
          { $inc: { stock_quantity: -item.quantity } }
        );
      }
    }

    res.status(201).json({ message: 'Order placed successfully', order: result.rows[0] });
  } catch (error) {
    res.status(400).json({ message: error.message });
  }
});

router.put('/orders/:id', auth, async (req, res) => {
  try {
    const { status, payment_status } = req.body;
    const orderId = req.params.id;

    let updates = [];
    let values = [];
    let idx = 1;

    if (status) {
      updates.push(`status = $${idx++}`);
      values.push(status);
    }
    if (payment_status) {
      updates.push(`payment_status = $${idx++}`);
      values.push(payment_status);
    }

    if (updates.length === 0) return res.status(400).json({ message: 'No updates provided' });

    values.push(orderId);
    const query = `UPDATE orders SET ${updates.join(', ')} WHERE id = $${idx} RETURNING *`;
    const result = await pgPool.query(query, values);

    if (result.rows.length === 0) return res.status(404).json({ message: 'Order not found' });
    res.json(result.rows[0]);
  } catch (error) {
    res.status(400).json({ message: error.message });
  }
});

// ========================
// REVIEWS
// ========================
router.get('/reviews', async (req, res) => {
  try {
    const { admin } = req.query;
    let query = admin === 'true' ? {} : { is_approved: true };
    const reviews = await Review.find(query).sort({ created_at: -1 });
    res.json(reviews);
  } catch (error) {
    res.status(500).json({ message: error.message });
  }
});

router.post('/reviews', async (req, res) => {
  try {
    const lastRev = await Review.findOne().sort({ id: -1 });
    const newId = lastRev ? lastRev.id + 1 : 1;

    const review = new Review({
      id: newId,
      ...req.body
    });

    await review.save();
    res.status(201).json({ message: 'Review submitted successfully! It will appear once approved.', review });
  } catch (error) {
    res.status(400).json({ message: error.message });
  }
});

router.put('/reviews/:id/approve', auth, async (req, res) => {
  try {
    const review = await Review.findOneAndUpdate({ id: req.params.id }, { is_approved: true }, { new: true });
    if (!review) return res.status(404).json({ message: 'Review not found' });

    // Update product average rating
    const allApproved = await Review.find({ product_id: review.product_id, is_approved: true });
    if (allApproved.length > 0) {
      const sum = allApproved.reduce((acc, r) => acc + r.rating, 0);
      const avg = (sum / allApproved.length).toFixed(1);
      await Product.findOneAndUpdate({ id: review.product_id }, { rating_avg: avg, rating_count: allApproved.length });
    }

    res.json(review);
  } catch (error) {
    res.status(400).json({ message: error.message });
  }
});

router.put('/reviews/:id/reject', auth, async (req, res) => {
  try {
    const review = await Review.findOneAndUpdate({ id: req.params.id }, { is_approved: false }, { new: true });
    if (!review) return res.status(404).json({ message: 'Review not found' });
    res.json(review);
  } catch (error) {
    res.status(400).json({ message: error.message });
  }
});

// ========================
// NEWSLETTER & CONTACTS (PostgreSQL)
// ========================
router.get('/newsletter', async (req, res) => {
  try {
    const { rows } = await pgPool.query('SELECT * FROM subscribers ORDER BY subscribed_at DESC');
    res.json(rows);
  } catch (error) {
    res.status(500).json({ message: error.message });
  }
});

router.post('/newsletter', async (req, res) => {
  try {
    const { name, email } = req.body;

    // check exists
    const existing = await pgPool.query('SELECT id FROM subscribers WHERE email = $1', [email]);
    if (existing.rows.length > 0) {
      return res.status(200).json({ message: 'Email is already subscribed!' });
    }

    const { rows } = await pgPool.query('INSERT INTO subscribers (name, email) VALUES ($1, $2) RETURNING *', [name, email]);
    res.status(201).json({ message: 'Subscribed to newsletter successfully!', subscriber: rows[0] });
  } catch (error) {
    res.status(400).json({ message: error.message });
  }
});

router.get('/contacts', async (req, res) => {
  try {
    const { rows } = await pgPool.query('SELECT * FROM messages ORDER BY created_at DESC');
    res.json(rows);
  } catch (error) {
    res.status(500).json({ message: error.message });
  }
});

router.post('/contacts', async (req, res) => {
  try {
    const { name, phone, email, message } = req.body;
    const { rows } = await pgPool.query('INSERT INTO messages (name, phone, email, message) VALUES ($1, $2, $3, $4) RETURNING *', [name, phone, email, message]);
    res.status(201).json({ message: 'Message sent successfully! We will get in touch soon.', inquiry: rows[0] });
  } catch (error) {
    res.status(400).json({ message: error.message });
  }
});

// ========================
// OTHERS
// ========================
router.post('/upload', auth, upload.single('image'), (req, res) => {
  if (!req.file) return res.status(400).json({ message: 'No image file provided' });
  const imageUrl = `${req.protocol}://${req.get('host')}/uploads/${req.file.filename}`;
  res.json({ url: imageUrl, filename: req.file.filename });
});

router.get('/analytics', auth, async (req, res) => {
  try {
    const totalOrdersResult = await pgPool.query('SELECT COUNT(*) FROM orders');
    const totalSalesResult = await pgPool.query("SELECT SUM(total_amount) FROM orders WHERE payment_status = 'paid' OR status = 'delivered'");
    const totalSubscribersResult = await pgPool.query('SELECT COUNT(*) FROM subscribers');
    const pendingMessagesResult = await pgPool.query("SELECT COUNT(*) FROM messages WHERE is_read = false");
    const recentOrdersResult = await pgPool.query('SELECT * FROM orders ORDER BY created_at DESC LIMIT 5');

    const totalProducts = await Product.countDocuments();
    const lowStockCount = await Product.countDocuments({ stock_quantity: { $lte: 5 } });

    const totalSales = parseFloat(totalSalesResult.rows[0].sum || 0);

    const monthlySales = [
      { name: 'Jan', sales: totalSales * 0.15 + 5000 },
      { name: 'Feb', sales: totalSales * 0.20 + 8000 },
      { name: 'Mar', sales: totalSales * 0.25 + 12000 },
      { name: 'Apr', sales: totalSales * 0.30 + 15000 },
      { name: 'May', sales: totalSales * 0.35 + 20000 },
      { name: 'Jun', sales: totalSales + 25000 }
    ];

    res.json({
      totalSales,
      totalOrders: parseInt(totalOrdersResult.rows[0].count, 10),
      totalProducts,
      totalSubscribers: parseInt(totalSubscribersResult.rows[0].count, 10),
      lowStockCount,
      pendingMessagesCount: parseInt(pendingMessagesResult.rows[0].count, 10),
      recentOrders: recentOrdersResult.rows,
      monthlySales
    });
  } catch (error) {
    res.status(500).json({ message: error.message });
  }
});

module.exports = router;
