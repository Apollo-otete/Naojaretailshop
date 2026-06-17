// Client API service to communicate with the Node.js Express backend
const BASE_URL = '';

async function request(endpoint, options = {}) {
  const url = `${BASE_URL}${endpoint}`;
  const headers = {
    'Content-Type': 'application/json',
    ...options.headers,
  };

  const config = {
    ...options,
    headers,
  };

  if (options.body && !(options.body instanceof FormData)) {
    config.body = JSON.stringify(options.body);
  } else if (options.body instanceof FormData) {
    // Let browser set the Content-Type boundary automatically
    delete config.headers['Content-Type'];
  }

  const response = await fetch(url, config);
  const data = await response.json();

  if (!response.ok) {
    throw new Error(data.message || 'Something went wrong');
  }

  return data;
}

export const api = {
  // Products
  getProducts: (categorySlug = '', search = '') => {
    const params = new URLSearchParams();
    if (categorySlug) params.append('category', categorySlug);
    if (search) params.append('search', search);
    return request(`/api/products?${params.toString()}`);
  },

  getProductBySlug: (slug) => {
    return request(`/api/products/${slug}`);
  },

  // Categories
  getCategories: () => {
    return request('/api/categories');
  },

  // Orders & Checkouts
  createCheckout: (orderData) => {
    return request('/api/orders', {
      method: 'POST',
      body: orderData,
    });
  },

  // Reviews
  getReviews: () => {
    return request('/api/reviews');
  },

  submitReview: (reviewData) => {
    return request('/api/reviews', {
      method: 'POST',
      body: reviewData,
    });
  },

  // Newsletter
  subscribeNewsletter: (name, email) => {
    return request('/api/newsletter', {
      method: 'POST',
      body: { name, email },
    });
  },

  // Contacts
  submitContactForm: (contactData) => {
    return request('/api/contacts', {
      method: 'POST',
      body: contactData,
    });
  },

  // Image Upload
  uploadImage: (file) => {
    const formData = new FormData();
    formData.append('image', file);
    return request('/api/upload', {
      method: 'POST',
      body: formData,
    });
  },

  // Admin Portal endpoints
  admin: {
    // Products CRUD
    getProducts: () => request('/api/products?admin=true'),
    addProduct: (product) => request('/api/products', { method: 'POST', body: product }),
    editProduct: (id, product) => request(`/api/products/${id}`, { method: 'PUT', body: product }),
    deleteProduct: (id) => request(`/api/products/${id}`, { method: 'DELETE' }),

    // Categories CRUD
    addCategory: (category) => request('/api/categories', { method: 'POST', body: category }),
    editCategory: (id, category) => request(`/api/categories/${id}`, { method: 'PUT', body: category }),
    deleteCategory: (id) => request(`/api/categories/${id}`, { method: 'DELETE' }),

    // Orders Management
    getOrders: () => request('/api/orders'),
    updateOrderStatus: (id, status) => request(`/api/orders/${id}`, { method: 'PUT', body: { status } }),

    // Reviews Management
    getReviews: () => request('/api/reviews?admin=true'),
    approveReview: (id) => request(`/api/reviews/${id}/approve`, { method: 'PUT' }),
    rejectReview: (id) => request(`/api/reviews/${id}/reject`, { method: 'PUT' }),

    // Newsletters
    getSubscribers: () => request('/api/newsletter'),

    // Contacts
    getMessages: () => request('/api/contacts'),

    // Analytics
    getAnalytics: () => request('/api/analytics'),
  }
};
