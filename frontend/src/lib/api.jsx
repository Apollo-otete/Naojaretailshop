// API client for the Naoja Go backend (real data, no mock/localStorage fallbacks).
// Uses /api/* (legacy-compatible, proxied to backend-go in dev) and real responses from Postgres.

const BASE_URL = import.meta.env.VITE_API_URL || '';

function getToken() {
  return sessionStorage.getItem('adminToken') || localStorage.getItem('adminToken') || '';
}

async function parseResponse(response) {
  if (response.status === 204) return null;
  const text = await response.text();
  if (!text) return null;
  try {
    return JSON.parse(text);
  } catch {
    return { message: text };
  }
}

async function request(endpoint, options = {}) {
  const url = `${BASE_URL}${endpoint}`;
  const token = getToken();
  const headers = {
    ...(token ? { Authorization: `Bearer ${token}` } : {}),
    ...(options.headers || {}),
  };

  const config = { credentials: 'include', ...options, headers };

  if (options.body && !(options.body instanceof FormData)) {
    config.headers['Content-Type'] = config.headers['Content-Type'] || 'application/json';
    config.body = typeof options.body === 'string' ? options.body : JSON.stringify(options.body);
  } else if (options.body instanceof FormData) {
    // Let browser set multipart boundary
    delete config.headers['Content-Type'];
  }

  const response = await fetch(url, config);
  const data = await parseResponse(response);

  if (!response.ok) {
    const msg =
      (data && (data.message || data.error || data.title)) ||
      (data && data.detail) ||
      `Request failed (${response.status})`;
    const err = new Error(msg);
    err.status = response.status;
    err.data = data;
    throw err;
  }

  return data;
}

export const api = {
  // Auth (real backend session)
  auth: {
    login: (credentials) => request('/api/auth/login', { method: 'POST', body: credentials }),
    logout: () => request('/api/v1/auth/logout', { method: 'DELETE' }).catch(() => null),
    getSession: () => request('/api/v1/auth/session'),
    changePassword: (data) => request('/api/auth/change-password', { method: 'PUT', body: data }),
  },

  // Products (real catalogue)
  getProducts: (categorySlug = '', search = '', page = null, limit = null) => {
    const params = new URLSearchParams();
    if (categorySlug) params.append('category', categorySlug);
    if (search) params.append('search', search);
    if (page) params.append('page', page);
    if (limit) params.append('limit', limit);
    const qs = params.toString();
    return request(`/api/products${qs ? `?${qs}` : ''}`);
  },

  getProductBySlug: (slug) => request(`/api/products/${encodeURIComponent(slug)}`),

  // Categories
  getCategories: () => request('/api/categories'),

  // Orders & checkout (server-authoritative; backend validates)
  createCheckout: (orderData) =>
    request('/api/orders', {
      method: 'POST',
      body: orderData,
    }),

  // Public verified order tracking (POST ref+phone; non-leaking)
  trackOrder: (ref, phone) =>
    request('/api/orders/track', {
      method: 'POST',
      body: { reference: ref, ref, phone },
    }),

  // M-Pesa (real flow when configured; backend never trusts client amount)
  mpesa: {
    stkPush: (data) => request('/api/mpesa/stkpush', { method: 'POST', body: data }),
    query: (data) =>
      request('/api/mpesa/query', {
        method: 'POST',
        body: typeof data === 'string' ? { checkoutRequestId: data } : data,
      }),
    getOrderStatus: (orderRef) => request(`/api/mpesa/order-status/${encodeURIComponent(orderRef)}`),
  },

  // Reviews / newsletter / contacts (served by backend when implemented; no mock fallback)
  getReviews: () => request('/api/reviews'),
  submitReview: (reviewData) => request('/api/reviews', { method: 'POST', body: reviewData }),
  subscribeNewsletter: (name, email) => request('/api/newsletter', { method: 'POST', body: { name, email } }),
  submitContactForm: (contactData) => request('/api/contacts', { method: 'POST', body: contactData }),

  // Image upload (Cloudinary via backend when configured)
  uploadImage: (file) => {
    const formData = new FormData();
    formData.append('file', file);
    formData.append('image', file);
    return request('/api/upload', { method: 'POST', body: formData });
  },

  // Admin endpoints (real backend; Bearer/session as supported)
  admin: {
    login: (credentials) => request('/api/auth/login', { method: 'POST', body: credentials }),
    changePassword: (data) => request('/api/auth/change-password', { method: 'PUT', body: data }),

    getProducts: () => request('/api/products?admin=true'),
    addProduct: (product) => request('/api/products', { method: 'POST', body: product }),
    editProduct: (id, product) => request(`/api/products/${id}`, { method: 'PUT', body: product }),
    deleteProduct: (id) => request(`/api/products/${id}`, { method: 'DELETE' }),

    addCategory: (category) => request('/api/categories', { method: 'POST', body: category }),
    editCategory: (id, category) => request(`/api/categories/${id}`, { method: 'PUT', body: category }),
    deleteCategory: (id) => request(`/api/categories/${id}`, { method: 'DELETE' }),

    getOrders: (page = null, limit = null) => {
      const params = new URLSearchParams();
      if (page) params.append('page', page);
      if (limit) params.append('limit', limit);
      const qs = params.toString();
      return request(`/api/orders${qs ? `?${qs}` : ''}`);
    },
    updateOrderStatus: (id, status) => request(`/api/orders/${id}`, { method: 'PUT', body: { status } }),
    updateOrderPaymentStatus: (id, paymentStatus) =>
      request(`/api/orders/${id}`, { method: 'PUT', body: { payment_status: paymentStatus } }),

    getReviews: () => request('/api/reviews?admin=true'),
    approveReview: (id) => request(`/api/reviews/${id}/approve`, { method: 'PUT' }),
    rejectReview: (id) => request(`/api/reviews/${id}/reject`, { method: 'PUT' }),
    getSubscribers: () => request('/api/newsletter'),
    getMessages: () => request('/api/contacts'),
    markMessageRead: (id) => request(`/api/contacts/${id}/read`, { method: 'PUT' }),
    getAnalytics: () => request('/api/analytics'),
  },
};
