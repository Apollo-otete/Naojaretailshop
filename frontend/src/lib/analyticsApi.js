// Analytics & Business Intelligence API client (real backend only; no mock/fake data).
// When the backend does not provide analytics data, return honest empty states (never fabricated metrics).

const API_BASE = import.meta.env.VITE_API_URL
  ? `${import.meta.env.VITE_API_URL}/api/analytics`
  : '/api/analytics';

const getHeaders = () => {
  const token = sessionStorage.getItem('adminToken') || localStorage.getItem('adminToken') || localStorage.getItem('naoja_token');
  return {
    'Content-Type': 'application/json',
    ...(token ? { Authorization: `Bearer ${token}` } : {}),
  };
};

const safeGet = async (path) => {
  const res = await fetch(`${API_BASE}${path}`, { headers: getHeaders(), credentials: 'include' });
  if (!res.ok) throw new Error(`analytics ${res.status}`);
  return res.json();
};

const safePost = async (path, body) => {
  const res = await fetch(`${API_BASE}${path}`, {
    method: 'POST',
    headers: getHeaders(),
    credentials: 'include',
    body: JSON.stringify(body || {}),
  });
  if (!res.ok) throw new Error(`analytics ${res.status}`);
  return res.json();
};

const emptyLive = () => ({
  revenueToday: 0,
  ordersToday: 0,
  paidOrdersToday: 0,
  ordersLast15Min: 0,
  inFlightStkPushes: 0,
  lowStockCount: 0,
  outOfStockCount: 0,
  successRate: 0,
  failedToday: 0,
  pendingDispatch: 0,
  unreadMessages: 0,
  recentEvents: [],
});

const emptySummary = (range = 'week') => ({
  range,
  summary: {
    totalRevenue: 0,
    revenueGrowth: 0,
    totalOrders: 0,
    ordersGrowth: 0,
    avgOrderValue: 0,
    aovGrowth: 0,
    conversionRate: 0,
    conversionGrowth: 0,
    returnCustomerRate: 0,
  },
  revenueTrend: [],
});

export const analyticsApi = {
  getLive: async () => {
    try { return await safeGet('/live'); } catch { return emptyLive(); }
  },

  getSummary: async (range = 'week', from = '', to = '') => {
    try {
      const params = new URLSearchParams({ range });
      if (from) params.append('from', from);
      if (to) params.append('to', to);
      return await safeGet(`/summary?${params.toString()}`);
    } catch {
      return emptySummary(range);
    }
  },

  getCategoryPerformance: async (range = 'month') => {
    try { return await safeGet(`/category-performance?range=${encodeURIComponent(range)}`); } catch { return []; }
  },

  getInventoryHealth: async () => {
    try { return await safeGet('/inventory/health'); } catch { return []; }
  },

  getRevenueForecast: async (horizon = 30) => {
    try { return await safeGet(`/forecast/revenue?horizon=${encodeURIComponent(horizon)}`); } catch {
      return { horizonDays: horizon, projectedTotal: 0, growthRate: 0, historicalDailyAvg: 0, forecast: [] };
    }
  },

  getInsights: async () => {
    try { return await safeGet('/forecast/insights'); } catch { return []; }
  },

  getPaymentHealth: async (range = 'month') => {
    try { return await safeGet(`/payment-health?range=${encodeURIComponent(range)}`); } catch {
      return { period: range, successRate: 0, totalAttempts: 0, completedCount: 0, failedCount: 0, pendingCount: 0, avgProcessingSec: 0, failureReasons: [] };
    }
  },

  getFulfillment: async (range = 'month') => {
    try { return await safeGet(`/fulfillment?range=${encodeURIComponent(range)}`); } catch {
      return {
        period: range,
        avgDispatchHours: 0,
        onTimeRate: 0,
        bottlenecksCount: 0,
        funnel: [],
        statusBreakdown: { pending: 0, confirmed: 0, processing: 0, dispatched: 0, delivered: 0, cancelled: 0 },
        deliverySplit: { pickup: 0, homeDelivery: 0, pickupPct: 0 },
      };
    }
  },

  getAlerts: async () => {
    try { return await safeGet('/alerts'); } catch { return []; }
  },

  acknowledgeAlert: async (id) => {
    try { return await safePost(`/alerts/${encodeURIComponent(id)}/acknowledge`); } catch { return { success: false, id, acknowledged: false }; }
  },

  getTargetProgress: async (period = 'month') => {
    try { return await safeGet(`/targets/progress?period=${encodeURIComponent(period)}`); } catch {
      return { period, revenueTarget: 0, revenueActual: 0, revenueProgress: 0, ordersTarget: 0, ordersActual: 0, ordersProgress: 0, daysRemainingInPeriod: 0, onTrack: false };
    }
  },

  setTarget: async (targetData) => {
    try { return await safePost('/targets', targetData); } catch { return { success: false }; }
  },

  getExportUrl: (type = 'orders', range = 'month') => {
    const token = sessionStorage.getItem('adminToken') || localStorage.getItem('adminToken') || localStorage.getItem('naoja_token') || '';
    return `${API_BASE}/export?type=${encodeURIComponent(type)}&range=${encodeURIComponent(range)}&token=${encodeURIComponent(token)}`;
  },

  getStreamUrl: () => {
    const token = sessionStorage.getItem('adminToken') || localStorage.getItem('adminToken') || localStorage.getItem('naoja_token') || '';
    return `${API_BASE}/events/stream?token=${encodeURIComponent(token)}`;
  },
};
