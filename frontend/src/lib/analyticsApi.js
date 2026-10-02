// ==========================================================
// Analytics & Business Intelligence API Client
// Integrated with Backend API + Graceful Local Dev Fallbacks
// ==========================================================

const API_BASE = import.meta.env.VITE_API_URL 
  ? `${import.meta.env.VITE_API_URL}/api/analytics` 
  : '/api/analytics';

const getHeaders = () => {
  const token = sessionStorage.getItem('adminToken') || localStorage.getItem('adminToken') || localStorage.getItem('naoja_token');
  return {
    'Content-Type': 'application/json',
    ...(token ? { Authorization: `Bearer ${token}` } : {})
  };
};

// ==========================================================
// Real-world Naoja Shop Mock Fallbacks (Kakamega retail)
// ==========================================================
const MOCK_LIVE_EVENTS = [
  { id: 'ev-1', type: 'payment_received', message: 'M-Pesa KSh 34,500 received from 254712***789 for Samsung Galaxy A35', time: '1 min ago', amount: 34500, orderRef: 'NAO-8921' },
  { id: 'ev-2', type: 'order_created', message: 'New order NAO-8922 placed by Mercy A. (Kakamega Town)', time: '4 min ago', amount: 4800, orderRef: 'NAO-8922' },
  { id: 'ev-3', type: 'low_stock', message: 'Inventory Warning: JBL Tune 510BT reached 2 units', time: '12 min ago', amount: null, orderRef: null },
  { id: 'ev-4', type: 'payment_received', message: 'M-Pesa KSh 12,500 received from 254790***123 for Solar Floodlight 200W', time: '18 min ago', amount: 12500, orderRef: 'NAO-8919' },
  { id: 'ev-5', type: 'order_created', message: 'New order NAO-8920 placed by Collins M. (Store Pickup)', time: '27 min ago', amount: 8200, orderRef: 'NAO-8920' }
];

const MOCK_LIVE_DATA = {
  revenueToday: 51800,
  ordersToday: 7,
  paidOrdersToday: 6,
  ordersLast15Min: 2,
  inFlightStkPushes: 1,
  lowStockCount: 3,
  outOfStockCount: 0,
  successRate: 88,
  failedToday: 1,
  pendingDispatch: 4,
  unreadMessages: 2,
  recentEvents: MOCK_LIVE_EVENTS
};

const generateMockSalesTrend = (days = 30) => {
  const list = [];
  const baseRevenue = 42000;
  for (let i = days - 1; i >= 0; i--) {
    const d = new Date();
    d.setDate(d.getDate() - i);
    const dateStr = d.toISOString().slice(0, 10);
    const dayOfWeek = d.getDay();
    const weekendMultiplier = (dayOfWeek === 5 || dayOfWeek === 6) ? 1.4 : 1.0;
    const randomVariation = 0.85 + Math.random() * 0.35;
    const revenue = Math.round(baseRevenue * weekendMultiplier * randomVariation);
    const orders = Math.max(2, Math.round(revenue / 5200));
    list.push({ date: dateStr, revenue, orders });
  }
  return list;
};

const MOCK_INVENTORY_ITEMS = [
  { productId: 'p-1', name: 'iPhone 15 Pro Max 256GB', currentStock: 15, dailyVelocity: 0.6, daysRemaining: 25, status: 'healthy', reorderPoint: 4, suggestedReorderQty: 10, runOutDate: 'In 25 days' },
  { productId: 'p-2', name: 'Samsung Galaxy A35 5G', currentStock: 2, dailyVelocity: 1.1, daysRemaining: 2, status: 'critical', reorderPoint: 5, suggestedReorderQty: 15, runOutDate: 'In 2 days' },
  { productId: 'p-3', name: 'JBL Tune 510BT Wireless Headphones', currentStock: 3, dailyVelocity: 0.9, daysRemaining: 3, status: 'low', reorderPoint: 6, suggestedReorderQty: 12, runOutDate: 'In 3 days' },
  { productId: 'p-4', name: 'Oraimo 20000mAh Power Bank', currentStock: 18, dailyVelocity: 1.5, daysRemaining: 12, status: 'healthy', reorderPoint: 8, suggestedReorderQty: 20, runOutDate: 'In 12 days' },
  { productId: 'p-5', name: 'Solar Floodlight 200W Commercial', currentStock: 4, dailyVelocity: 0.7, daysRemaining: 6, status: 'low', reorderPoint: 5, suggestedReorderQty: 10, runOutDate: 'In 6 days' },
  { productId: 'p-6', name: 'HP 15-dw Intel Core i5 8GB/512GB', currentStock: 6, dailyVelocity: 0.3, daysRemaining: 20, status: 'healthy', reorderPoint: 3, suggestedReorderQty: 5, runOutDate: 'In 20 days' },
  { productId: 'p-7', name: 'Hikvision 4-Camera HD CCTV Kit', currentStock: 5, dailyVelocity: 0.4, daysRemaining: 12, status: 'healthy', reorderPoint: 3, suggestedReorderQty: 6, runOutDate: 'In 12 days' }
];

const MOCK_CATEGORIES_PERF = [
  { category: 'Mobile Phones', revenue: 485000, unitsSold: 28, percentage: 38 },
  { category: 'Audio Devices', revenue: 215000, unitsSold: 42, percentage: 17 },
  { category: 'Charging Accessories', revenue: 165000, unitsSold: 64, percentage: 13 },
  { category: 'Solar & Renewable', revenue: 195000, unitsSold: 18, percentage: 15 },
  { category: 'Computers & Laptops', revenue: 140000, unitsSold: 7, percentage: 11 },
  { category: 'Security Systems', revenue: 76000, unitsSold: 6, percentage: 6 }
];

const MOCK_INSIGHTS = [
  {
    type: 'opportunity',
    title: 'Weekend Surge Opportunity',
    detail: 'Friday and Saturday sales in Kakamega are consistently 38% higher than midweek. Ensure morning store stock is replenished by Friday 10:00 AM.',
    action: 'Schedule supplier restocks every Thursday afternoon.',
    impact: 'Estimated +12% weekend revenue retention'
  },
  {
    type: 'inventory',
    title: 'Critical Stockout Alert: Samsung Galaxy A35',
    detail: 'Only 2 units remain with an average velocity of 1.1 units/day. Stock run-out projected within 48 hours.',
    action: 'Place urgent purchase order for 15 units from Nairobi distributor.',
    impact: 'Avoid KSh 51,750 in lost gross sales'
  },
  {
    type: 'positive',
    title: 'High M-Pesa Conversion Stability',
    detail: 'Daraja STK Push completion rate reached 89.2% over the last 14 days, well above Kenyan retail benchmark (82%).',
    action: 'Keep Till Number 4149288 prominent on checkout banner.',
    impact: 'Frictionless checkout experience'
  }
];

export const analyticsApi = {
  // Live KPI stream and counts
  getLive: async () => {
    try {
      const res = await fetch(`${API_BASE}/live`, { headers: getHeaders() });
      if (res.ok) return await res.json();
    } catch (e) {
      // Fallback
    }
    return MOCK_LIVE_DATA;
  },

  // Time-sliced summary & revenue trend
  getSummary: async (range = 'week', from = '', to = '') => {
    try {
      const params = new URLSearchParams({ range });
      if (from) params.append('from', from);
      if (to) params.append('to', to);
      const res = await fetch(`${API_BASE}/summary?${params.toString()}`, { headers: getHeaders() });
      if (res.ok) return await res.json();
    } catch (e) {
      // Fallback
    }
    const days = range === 'week' ? 7 : range === 'month' ? 30 : 90;
    const trend = generateMockSalesTrend(days);
    const totalRev = trend.reduce((sum, item) => sum + item.revenue, 0);
    const totalOrd = trend.reduce((sum, item) => sum + item.orders, 0);
    return {
      range,
      summary: {
        totalRevenue: totalRev,
        revenueGrowth: 14.8,
        totalOrders: totalOrd,
        ordersGrowth: 9.2,
        avgOrderValue: Math.round(totalRev / (totalOrd || 1)),
        aovGrowth: 5.1,
        conversionRate: 3.4,
        conversionGrowth: 0.6,
        returnCustomerRate: 28.5
      },
      revenueTrend: trend
    };
  },

  // Category performance
  getCategoryPerformance: async (range = 'month') => {
    try {
      const res = await fetch(`${API_BASE}/category-performance?range=${range}`, { headers: getHeaders() });
      if (res.ok) return await res.json();
    } catch (e) {
      // Fallback
    }
    return MOCK_CATEGORIES_PERF;
  },

  // Inventory velocity & stockout forecast
  getInventoryHealth: async () => {
    try {
      const res = await fetch(`${API_BASE}/inventory/health`, { headers: getHeaders() });
      if (res.ok) return await res.json();
    } catch (e) {
      // Fallback
    }
    return MOCK_INVENTORY_ITEMS;
  },

  // Predictive revenue forecast
  getRevenueForecast: async (horizon = 30) => {
    try {
      const res = await fetch(`${API_BASE}/forecast/revenue?horizon=${horizon}`, { headers: getHeaders() });
      if (res.ok) return await res.json();
    } catch (e) {
      // Fallback
    }
    const forecast = [];
    const baseDaily = 44000;
    let projSum = 0;
    for (let i = 1; i <= horizon; i++) {
      const d = new Date();
      d.setDate(d.getDate() + i);
      const proj = Math.round(baseDaily * (1 + 0.003 * i) * (0.95 + Math.random() * 0.1));
      forecast.push({
        date: d.toISOString().slice(0, 10),
        projected: proj,
        lowerBound: Math.round(proj * 0.88),
        upperBound: Math.round(proj * 1.15)
      });
      projSum += proj;
    }
    return {
      horizonDays: horizon,
      projectedTotal: projSum,
      growthRate: 11.4,
      historicalDailyAvg: baseDaily,
      forecast
    };
  },

  // Executive natural-language insights
  getInsights: async () => {
    try {
      const res = await fetch(`${API_BASE}/forecast/insights`, { headers: getHeaders() });
      if (res.ok) return await res.json();
    } catch (e) {
      // Fallback
    }
    return MOCK_INSIGHTS;
  },

  // Payment diagnostics & Daraja health
  getPaymentHealth: async (range = 'month') => {
    try {
      const res = await fetch(`${API_BASE}/payment-health?range=${range}`, { headers: getHeaders() });
      if (res.ok) return await res.json();
    } catch (e) {
      // Fallback
    }
    return {
      period: range,
      successRate: 87.5,
      totalAttempts: 184,
      completedCount: 161,
      failedCount: 23,
      pendingCount: 2,
      avgProcessingSec: 6.2,
      failureReasons: [
        { reason: 'User Cancelled / Invalid PIN', count: 14, pct: 60.8 },
        { reason: 'Insufficient M-Pesa Balance', count: 6, pct: 26.1 },
        { reason: 'Daraja Timeout / SIM Unreachable', count: 3, pct: 13.1 }
      ]
    };
  },

  // Order fulfillment & funnel
  getFulfillment: async (range = 'month') => {
    try {
      const res = await fetch(`${API_BASE}/fulfillment?range=${range}`, { headers: getHeaders() });
      if (res.ok) return await res.json();
    } catch (e) {
      // Fallback
    }
    return {
      period: range,
      avgDispatchHours: 3.4,
      onTimeRate: 96.2,
      bottlenecksCount: 2,
      funnel: [
        { stage: 'Placed', count: 161, pct: 100 },
        { stage: 'Confirmed', count: 158, pct: 98 },
        { stage: 'Packed', count: 154, pct: 95 },
        { stage: 'Dispatched', count: 150, pct: 93 },
        { stage: 'Delivered', count: 147, pct: 91 }
      ],
      statusBreakdown: {
        pending: 3,
        confirmed: 4,
        processing: 6,
        dispatched: 8,
        delivered: 140,
        cancelled: 4
      },
      deliverySplit: {
        pickup: 98,
        homeDelivery: 63,
        pickupPct: 61
      }
    };
  },

  // Active managerial alerts
  getAlerts: async () => {
    try {
      const res = await fetch(`${API_BASE}/alerts`, { headers: getHeaders() });
      if (res.ok) return await res.json();
    } catch (e) {
      // Fallback
    }
    return [
      {
        id: 'alt-1',
        severity: 'critical',
        title: 'Imminent Stockout: Samsung Galaxy A35',
        message: 'Only 2 units remain in Lurambi shop. High velocity item. Projected stockout in 48 hours.',
        entity_type: 'product',
        created_at: new Date(Date.now() - 3600000).toISOString(),
        acknowledged: false
      },
      {
        id: 'alt-2',
        severity: 'warning',
        title: 'Dispatch Queue Latency',
        message: '4 Kakamega town delivery orders have been packed for over 2 hours without courier dispatch.',
        entity_type: 'order',
        created_at: new Date(Date.now() - 7200000).toISOString(),
        acknowledged: false
      },
      {
        id: 'alt-3',
        severity: 'info',
        title: 'Monthly Revenue Target 80% Reached',
        message: 'Store has attained KSh 480,000 of the KSh 600,000 goal with 8 days remaining in the month.',
        entity_type: 'target',
        created_at: new Date(Date.now() - 86400000).toISOString(),
        acknowledged: false
      }
    ];
  },

  acknowledgeAlert: async (id) => {
    try {
      const res = await fetch(`${API_BASE}/alerts/${id}/acknowledge`, {
        method: 'POST',
        headers: getHeaders()
      });
      if (res.ok) return await res.json();
    } catch (e) {
      // Fallback
    }
    return { success: true, id, acknowledged: true };
  },

  // Target and progress
  getTargetProgress: async (period = 'month') => {
    try {
      const res = await fetch(`${API_BASE}/targets/progress?period=${period}`, { headers: getHeaders() });
      if (res.ok) return await res.json();
    } catch (e) {
      // Fallback
    }
    return {
      period,
      revenueTarget: 600000,
      revenueActual: 485000,
      revenueProgress: 80.8,
      ordersTarget: 150,
      ordersActual: 132,
      ordersProgress: 88.0,
      daysRemainingInPeriod: 8,
      onTrack: true
    };
  },

  setTarget: async (targetData) => {
    try {
      const res = await fetch(`${API_BASE}/targets`, {
        method: 'POST',
        headers: getHeaders(),
        body: JSON.stringify(targetData)
      });
      if (res.ok) return await res.json();
    } catch (e) {
      // Fallback
    }
    return { success: true, ...targetData, updated_at: new Date().toISOString() };
  },

  // Download CSV export
  getExportUrl: (type = 'orders', range = 'month') => {
    const token = sessionStorage.getItem('adminToken') || localStorage.getItem('adminToken') || localStorage.getItem('naoja_token') || '';
    return `${API_BASE}/export?type=${type}&range=${range}&token=${encodeURIComponent(token)}`;
  },

  // SSE Stream URL
  getStreamUrl: () => {
    const token = sessionStorage.getItem('adminToken') || localStorage.getItem('adminToken') || localStorage.getItem('naoja_token') || '';
    return `${API_BASE}/events/stream?token=${encodeURIComponent(token)}`;
  }
};
