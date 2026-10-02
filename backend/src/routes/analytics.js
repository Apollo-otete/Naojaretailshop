// ==========================================================
// Analytics & Business Intelligence API Routes
// Mount at /api/analytics
// ==========================================================

const express = require('express');
const router = express.Router();
const jwt = require('jsonwebtoken');
const authMiddleware = require('../middleware/auth');
const eventBus = require('../services/eventBus');
const analyticsService = require('../services/analyticsService');
const { generateRevenueForecast, generateExecutiveInsights } = require('../services/forecastService');

// Middleware supporting token via header OR query parameter for SSE EventSource
const sseAuthMiddleware = (req, res, next) => {
  const token = req.query.token || (req.header('Authorization') ? req.header('Authorization').replace('Bearer ', '') : null);

  if (!token) {
    return res.status(401).json({ message: 'No authentication token, authorization denied' });
  }

  try {
    const jwtSecret = process.env.JWT_SECRET || 'fallback_dev_secret_key_123';
    const verified = jwt.verify(token, jwtSecret);
    req.admin = verified;
    next();
  } catch (err) {
    res.status(401).json({ message: 'Token verification failed' });
  }
};

/**
 * 🟢 GET /api/analytics/events/stream
 * Real-time Server-Sent Events stream for connected Admin Dashboard
 */
router.get('/events/stream', sseAuthMiddleware, (req, res) => {
  res.writeHead(200, {
    'Content-Type': 'text/event-stream',
    'Cache-Control': 'no-cache',
    'Connection': 'keep-alive',
    'X-Accel-Buffering': 'no'
  });

  res.write(`data: ${JSON.stringify({ type: 'connected', message: 'Connected to Naoja Live Business Stream' })}\n\n`);

  eventBus.addClient(res);
});

/**
 * 🟢 GET /api/analytics/live
 * Live KPI counts: today's revenue, in-flight STK, orders last 15 min, recent events
 */
router.get('/live', authMiddleware, async (req, res) => {
  try {
    const stats = await analyticsService.getLiveStats();
    res.json(stats);
  } catch (err) {
    res.status(500).json({ message: err.message });
  }
});

/**
 * 📊 GET /api/analytics/summary
 * Time-sliced metrics: revenue, orders, AOV, units, conversion, trend
 */
router.get('/summary', authMiddleware, async (req, res) => {
  try {
    const { range = 'week', from, to } = req.query;
    const summary = await analyticsService.getSummaryAnalytics(range, from, to);
    res.json(summary);
  } catch (err) {
    res.status(500).json({ message: err.message });
  }
});

/**
 * 📊 GET /api/analytics/category-performance
 */
router.get('/category-performance', authMiddleware, async (req, res) => {
  try {
    const { range = 'month' } = req.query;
    const data = await analyticsService.getCategoryPerformance(range);
    res.json(data);
  } catch (err) {
    res.status(500).json({ message: err.message });
  }
});

/**
 * 📦 GET /api/analytics/inventory/health
 * Stock levels, sales velocity, predicted stock-out date, reorder suggestions
 */
router.get('/inventory/health', authMiddleware, async (req, res) => {
  try {
    const inventory = await analyticsService.getInventoryHealth();
    res.json(inventory);
  } catch (err) {
    res.status(500).json({ message: err.message });
  }
});

/**
 * 🔮 GET /api/analytics/forecast/revenue
 * 7, 30, 90, 365 day revenue projections with confidence intervals
 */
router.get('/forecast/revenue', authMiddleware, async (req, res) => {
  try {
    const { horizon = 30 } = req.query;
    const summary = await analyticsService.getSummaryAnalytics('month');
    const forecast = generateRevenueForecast(summary.revenueTrend, Number(horizon));
    res.json(forecast);
  } catch (err) {
    res.status(500).json({ message: err.message });
  }
});

/**
 * 🔮 GET /api/analytics/forecast/insights
 * AI/Statistical natural-language executive managerial insights
 */
router.get('/forecast/insights', authMiddleware, async (req, res) => {
  try {
    const [summaryData, categories, inventory, payment, targets] = await Promise.all([
      analyticsService.getSummaryAnalytics('week'),
      analyticsService.getCategoryPerformance('week'),
      analyticsService.getInventoryHealth(),
      analyticsService.getPaymentHealth('week'),
      analyticsService.getTargetsProgress('month')
    ]);

    const insights = generateExecutiveInsights({
      summary: summaryData.summary,
      revenueTrend: summaryData.revenueTrend,
      categoryPerformance: categories,
      inventoryAlerts: inventory,
      paymentHealth: payment,
      targets
    });

    res.json(insights);
  } catch (err) {
    res.status(500).json({ message: err.message });
  }
});

/**
 * 💳 GET /api/analytics/payment-health
 */
router.get('/payment-health', authMiddleware, async (req, res) => {
  try {
    const { range = 'month' } = req.query;
    const data = await analyticsService.getPaymentHealth(range);
    res.json(data);
  } catch (err) {
    res.status(500).json({ message: err.message });
  }
});

/**
 * 🚚 GET /api/analytics/fulfillment
 */
router.get('/fulfillment', authMiddleware, async (req, res) => {
  try {
    const { range = 'month' } = req.query;
    const data = await analyticsService.getFulfillmentAnalytics(range);
    res.json(data);
  } catch (err) {
    res.status(500).json({ message: err.message });
  }
});

/**
 * 🚨 GET /api/analytics/alerts
 */
router.get('/alerts', authMiddleware, async (req, res) => {
  try {
    const alerts = await analyticsService.getActiveAlerts();
    res.json(alerts);
  } catch (err) {
    res.status(500).json({ message: err.message });
  }
});

/**
 * 🚨 POST /api/analytics/alerts/:id/acknowledge
 */
router.post('/alerts/:id/acknowledge', authMiddleware, async (req, res) => {
  try {
    const adminEmail = req.admin?.email || 'Admin';
    await analyticsService.acknowledgeAlert(req.params.id, adminEmail);
    res.json({ success: true, message: 'Alert acknowledged' });
  } catch (err) {
    res.status(500).json({ message: err.message });
  }
});

/**
 * 🎯 GET /api/analytics/targets/progress
 */
router.get('/targets/progress', authMiddleware, async (req, res) => {
  try {
    const { period = 'month' } = req.query;
    const progress = await analyticsService.getTargetsProgress(period);
    res.json(progress);
  } catch (err) {
    res.status(500).json({ message: err.message });
  }
});

/**
 * 🎯 POST /api/analytics/targets
 */
router.post('/targets', authMiddleware, async (req, res) => {
  try {
    const { period = 'month', periodStart, revenueTarget, ordersTarget } = req.body;
    await analyticsService.setTarget(period, periodStart, revenueTarget, ordersTarget);
    res.json({ success: true, message: 'Target saved successfully' });
  } catch (err) {
    res.status(500).json({ message: err.message });
  }
});

/**
 * 📥 GET /api/analytics/export
 * One-click CSV export for offline planning
 */
router.get('/export', authMiddleware, async (req, res) => {
  try {
    const { type = 'orders', range = 'month' } = req.query;

    if (type === 'inventory') {
      const inventory = await analyticsService.getInventoryHealth();
      let csv = 'Product ID,Name,Current Stock,Daily Velocity,Days Remaining,Status,Reorder Point,Suggested Reorder\n';
      inventory.forEach(item => {
        csv += `"${item.productId}","${item.name.replace(/"/g, '""')}",${item.currentStock},${item.dailyVelocity},${item.daysRemaining},"${item.status}",${item.reorderPoint},${item.suggestedReorderQty}\n`;
      });

      res.setHeader('Content-Type', 'text/csv');
      res.setHeader('Content-Disposition', `attachment; filename=naoja_inventory_${Date.now()}.csv`);
      return res.send(csv);
    }

    // Default orders export
    const { currentStart, currentEnd } = analyticsService.getPeriodBoundaries ? analyticsService.getPeriodBoundaries(range) : { currentStart: new Date(Date.now() - 30 * 86400000), currentEnd: new Date() };
    const { pgPool } = require('../config/db');
    const ordersRes = await pgPool.query('SELECT * FROM orders ORDER BY created_at DESC LIMIT 500');

    let csv = 'Order Ref,Customer Name,Phone,Email,Total (KES),Payment Status,Order Status,Created At\n';
    ordersRes.rows.forEach(o => {
      csv += `"${o.order_ref}","${(o.customer_name || '').replace(/"/g, '""')}","${o.customer_phone || ''}","${o.customer_email || ''}",${o.total_amount},"${o.payment_status}","${o.status}","${o.created_at}"\n`;
    });

    res.setHeader('Content-Type', 'text/csv');
    res.setHeader('Content-Disposition', `attachment; filename=naoja_orders_${Date.now()}.csv`);
    return res.send(csv);
  } catch (err) {
    res.status(500).json({ message: err.message });
  }
});

module.exports = router;
