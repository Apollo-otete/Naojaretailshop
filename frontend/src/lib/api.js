// Client API service to communicate with the Node.js Express backend.
// Falls back to a local storage database if the server is offline or unavailable.

const BASE_URL = '';

// Default Mock Data for Kakamega Naoja Ventures Retail Shop
const DEFAULT_CATEGORIES = [
  { id: 1, name: 'Mobile Phones', slug: 'mobile-phones', icon: '📱', product_count: 3 },
  { id: 2, name: 'Audio Devices', slug: 'audio-devices', icon: '🎧', product_count: 2 },
  { id: 3, name: 'Television Products', slug: 'television-products', icon: '📺', product_count: 1 },
  { id: 4, name: 'Charging Accessories', slug: 'charging-accessories', icon: '🔌', product_count: 1 },
  { id: 5, name: 'Computers & Accessories', slug: 'computers-accessories', icon: '💻', product_count: 1 },
  { id: 6, name: 'Electrical Products', slug: 'electrical-products', icon: '⚡', product_count: 1 },
  { id: 7, name: 'Security Products', slug: 'security-products', icon: '🔒', product_count: 1 },
  { id: 8, name: 'Networking Products', slug: 'networking-products', icon: '🌐', product_count: 1 },
  { id: 9, name: 'Automotive Products', slug: 'automotive-products', icon: '🚗', product_count: 0 },
  { id: 10, name: 'Gaming Products', slug: 'gaming-products', icon: '🎮', product_count: 0 },
  { id: 11, name: 'Watches', slug: 'watches', icon: '⌚', product_count: 0 },
  { id: 12, name: 'Renewable Energy', slug: 'renewable-energy', icon: '☀️', product_count: 1 },
  { id: 13, name: 'Home Appliances', slug: 'home-appliances', icon: '🏠', product_count: 1 },
  { id: 14, name: 'Other Electrical Products', slug: 'other-electrical', icon: '🔧', product_count: 0 }
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
  },
  {
    id: 2,
    name: 'Samsung Galaxy A35 5G',
    slug: 'samsung-galaxy-a35-5g',
    description: 'Samsung Galaxy A35 5G comes with a 6.6-inch Super AMOLED 120Hz display, a powerful 50MP triple-camera setup, Exynos 1380 processor, and a massive 5000mAh battery with 25W fast charging support.',
    price: 34500.00,
    stock_quantity: 2, // low stock test
    category_id: 1,
    subcategory: 'Smartphones',
    images: ['https://images.unsplash.com/photo-1610945265064-0e34e5519bbf?w=600&auto=format&fit=crop&q=80'],
    status: 'low_stock',
    is_featured: true,
    views: 189,
    rating_avg: 4.5,
    rating_count: 4
  },
  {
    id: 3,
    name: 'JBL Tune 510BT Wireless Headphones',
    slug: 'jbl-tune-510bt-wireless-headphones',
    description: 'The JBL Tune 510BT headphones let you stream powerful JBL Pure Bass sound with no strings attached. Easy to use, these headphones provide up to 40 hours of pure pleasure and an extra 2 hours of battery with just 5 minutes of power with the USB-C charging cable.',
    price: 6500.00,
    stock_quantity: 30,
    category_id: 2,
    subcategory: 'Wireless Headphones',
    images: ['https://images.unsplash.com/photo-1505740420928-5e560c06d30e?w=600&auto=format&fit=crop&q=80'],
    status: 'in_stock',
    is_featured: true,
    views: 245,
    rating_avg: 4.6,
    rating_count: 8
  },
  {
    id: 4,
    name: 'Oraimo FreePods 4 Active Noise Cancelling',
    slug: 'oraimo-freepods-4-anc',
    description: 'Oraimo FreePods 4 features active noise cancellation, heavy bass, IPX5 waterproof design, up to 35.5-hour playtime with the charging case, and convenient touch controls. Connects via Bluetooth 5.2.',
    price: 4200.00,
    stock_quantity: 0, // out of stock test
    category_id: 2,
    subcategory: 'Earbuds',
    images: ['https://images.unsplash.com/photo-1590658268037-6bf12165a8df?w=600&auto=format&fit=crop&q=80'],
    status: 'out_of_stock',
    is_featured: false,
    views: 310,
    rating_avg: 4.2,
    rating_count: 5
  },
  {
    id: 5,
    name: 'LG 55" UHD 4K Smart TV',
    slug: 'lg-55-uhd-4k-smart-tv',
    description: 'Experience crystal clear 4K resolution on this LG 55-inch Smart TV. Features ThinQ AI technology, WebOS smart portal for Netflix and YouTube, HDR10 Pro, and rich virtual surround sound.',
    price: 68000.00,
    stock_quantity: 6,
    category_id: 3,
    subcategory: 'Smart TVs',
    images: ['https://images.unsplash.com/photo-1593305841991-05c297ba4575?w=600&auto=format&fit=crop&q=80'],
    status: 'in_stock',
    is_featured: true,
    views: 154,
    rating_avg: 4.7,
    rating_count: 3
  },
  {
    id: 6,
    name: 'Oraimo 20000mAh Power Bank 12W',
    slug: 'oraimo-20000mah-power-bank-12w',
    description: 'High capacity 20,000mAh portable charger from Oraimo. Offers 12W fast charging, dual USB output ports, Type-C & Micro-USB inputs, and multi-protection safety system to protect your devices.',
    price: 3200.00,
    stock_quantity: 45,
    category_id: 4,
    subcategory: 'Power Banks',
    images: ['https://images.unsplash.com/photo-1609592424085-f509e51c8680?w=600&auto=format&fit=crop&q=80'],
    status: 'in_stock',
    is_featured: true,
    views: 520,
    rating_avg: 4.4,
    rating_count: 14
  },
  {
    id: 7,
    name: 'HP 250 G9 Intel Core i5 Laptop',
    slug: 'hp-250-g9-intel-core-i5-laptop',
    description: 'The HP 250 G9 Laptop keeps up with mobile workstyles with a thin and light design and a great screen-to-body ratio. Complete business tasks with a powerful Intel Core i5 12th Gen processor, 8GB DDR4 RAM, and 512GB PCIe NVMe SSD.',
    price: 58000.00,
    stock_quantity: 8,
    category_id: 5,
    subcategory: 'Laptops',
    images: ['https://images.unsplash.com/photo-1588872657578-7efd1f1555ed?w=600&auto=format&fit=crop&q=80'],
    status: 'in_stock',
    is_featured: true,
    views: 290,
    rating_avg: 4.5,
    rating_count: 6
  },
  {
    id: 8,
    name: 'Solar King 200W Mono Solar Panel',
    slug: 'solar-king-200w-mono-solar-panel',
    description: 'High-efficiency 200W Monocrystalline solar panel by Solar King. Designed for off-grid power systems, home backup systems, and water pumping. Features heavy-duty anodized aluminum frames and bypass diodes.',
    price: 12500.00,
    stock_quantity: 12,
    category_id: 12,
    subcategory: 'Solar Panels',
    images: ['https://images.unsplash.com/photo-1509391366360-2e959784a276?w=600&auto=format&fit=crop&q=80'],
    status: 'in_stock',
    is_featured: true,
    views: 180,
    rating_avg: 4.7,
    rating_count: 5
  },
  {
    id: 9,
    name: 'Ramtons Double Door Fridge 180L',
    slug: 'ramtons-double-door-fridge-180l',
    description: 'Keep your foods and drinks fresh with the Ramtons 180 Liters Double Door refrigerator. It features adjustable glass shelves, vegetable crisper box, freezer compartment, and energy-saving silent compressor.',
    price: 45000.00,
    stock_quantity: 4,
    category_id: 13,
    subcategory: 'Refrigerators',
    images: ['https://images.unsplash.com/photo-1584622650111-993a426fbf0a?w=600&auto=format&fit=crop&q=80'],
    status: 'low_stock',
    is_featured: false,
    views: 95,
    rating_avg: 4.3,
    rating_count: 2
  },
  {
    id: 10,
    name: 'MK 1 Way Single Switch White',
    slug: 'mk-1-way-single-switch-white',
    description: 'Genuine British Standard MK 1 Way single gang flush light switch. Made of premium urea formaldehyde polymer which has inherent antimicrobial properties. Highly durable for home or office electrical wiring.',
    price: 250.00,
    stock_quantity: 140,
    category_id: 6,
    subcategory: 'Wall Switches',
    images: ['https://images.unsplash.com/photo-1618220179428-22790b461013?w=600&auto=format&fit=crop&q=80'],
    status: 'in_stock',
    is_featured: false,
    views: 75,
    rating_avg: 4.9,
    rating_count: 9
  },
  {
    id: 11,
    name: 'Hikvision 2MP Security Dome Camera',
    slug: 'hikvision-2mp-security-dome-camera',
    description: 'High-quality imaging with 2 MP resolution, 1080p video recording, efficient H.265 compression technology, clear imaging against strong back light due to DWDR technology, and smart IR night vision up to 30m.',
    price: 3800.00,
    stock_quantity: 20,
    category_id: 7,
    subcategory: 'CCTV Cameras',
    images: ['https://images.unsplash.com/photo-1557597774-9d273605dfa9?w=600&auto=format&fit=crop&q=80'],
    status: 'in_stock',
    is_featured: false,
    views: 110,
    rating_avg: 4.6,
    rating_count: 3
  },
  {
    id: 12,
    name: 'TP-Link TL-WR840N 300Mbps Router',
    slug: 'tp-link-tl-wr840n-300mbps-router',
    description: 'TP-Link TL-WR840N is a combined wired/wireless network connection device designed specifically for small business and home office networking requirements. Delivers exceptional 300Mbps wireless performance.',
    price: 2200.00,
    stock_quantity: 35,
    category_id: 8,
    subcategory: 'Wi-Fi Routers',
    images: ['https://images.unsplash.com/photo-1544197150-b99a580bb7a8?w=600&auto=format&fit=crop&q=80'],
    status: 'in_stock',
    is_featured: false,
    views: 132,
    rating_avg: 4.5,
    rating_count: 6
  }
];

const DEFAULT_ORDERS = [
  {
    id: 1,
    order_ref: 'NJ-2026-0001',
    customer_name: 'John Doe',
    customer_phone: '0712345678',
    customer_email: 'john@example.com',
    total_amount: 9700.00,
    status: 'pending',
    payment_method: 'mpesa',
    payment_status: 'pending',
    mpesa_till_number: '4149288',
    mpesa_transaction_id: '',
    shipping_address: 'Kakamega, Amalemba estate near primary school',
    notes: 'Please call before delivering.',
    created_at: new Date(Date.now() - 3600000 * 2).toISOString(), // 2 hours ago
    items: [
      { id: 1, product_id: 3, product_name: 'JBL Tune 510BT Wireless Headphones', quantity: 1, unit_price: 6500.00, total_price: 6500.00 },
      { id: 2, product_id: 6, product_name: 'Oraimo 20000mAh Power Bank 12W', quantity: 1, unit_price: 3200.00, total_price: 3200.00 }
    ]
  },
  {
    id: 2,
    order_ref: 'NJ-2026-0002',
    customer_name: 'Mary Wambui',
    customer_phone: '0722334455',
    customer_email: 'mary@gmail.com',
    total_amount: 58000.00,
    status: 'delivered',
    payment_method: 'mpesa',
    payment_status: 'paid',
    mpesa_till_number: '4149288',
    mpesa_transaction_id: 'RFT34HG76D',
    shipping_address: 'MMUST Main Campus, Hall 3 Room 12',
    notes: 'Leave at the security desk if I am not in class.',
    created_at: new Date(Date.now() - 3600000 * 24).toISOString(), // 1 day ago
    items: [
      { id: 3, product_id: 7, product_name: 'HP 250 G9 Intel Core i5 Laptop', quantity: 1, unit_price: 58000.00, total_price: 58000.00 }
    ]
  },
  {
    id: 3,
    order_ref: 'NJ-2026-0003',
    customer_name: 'David Otieno',
    customer_phone: '0799887766',
    customer_email: 'david@otieno.me',
    total_amount: 25000.00,
    status: 'confirmed',
    payment_method: 'mpesa',
    payment_status: 'paid',
    mpesa_till_number: '4149288',
    mpesa_transaction_id: 'QWE98UY54P',
    shipping_address: 'Kakamega Town, Opposite KCB Bank',
    notes: 'I will pick it up from the shop.',
    created_at: new Date(Date.now() - 3600000 * 48).toISOString(), // 2 days ago
    items: [
      { id: 4, product_id: 8, product_name: 'Solar King 200W Mono Solar Panel', quantity: 2, unit_price: 12500.00, total_price: 25000.00 }
    ]
  }
];

const DEFAULT_REVIEWS = [
  {
    id: 1,
    product_id: 1,
    name: 'Alex K.',
    rating: 5,
    comment: 'Absolutely brilliant! Genuine product and fast delivery in Kakamega.',
    is_approved: true,
    created_at: new Date(Date.now() - 3600000 * 5).toISOString()
  },
  {
    id: 2,
    product_id: 6,
    name: 'Jane M.',
    rating: 4,
    comment: 'Lasts long and charges my phone multiple times. Satisfied with the power bank.',
    is_approved: true,
    created_at: new Date(Date.now() - 3600000 * 20).toISOString()
  },
  {
    id: 3,
    product_id: 7,
    name: 'Peter O.',
    rating: 5,
    comment: 'Great laptop for student work. Price was best in town. Highly recommend Naoja Ventures.',
    is_approved: false, // starts pending approval
    created_at: new Date(Date.now() - 3600000 * 30).toISOString()
  }
];

const DEFAULT_SUBSCRIBERS = [
  { id: 1, name: 'Naoja Admin', email: 'admin@naojaventures.com', is_active: true, subscribed_at: new Date().toISOString() },
  { id: 2, name: 'Visitor', email: 'info@naojaventures.com', is_active: true, subscribed_at: new Date().toISOString() },
  { id: 3, name: 'Samuel Wekesa', email: 'samuel@gmail.com', is_active: true, subscribed_at: new Date().toISOString() }
];

const DEFAULT_MESSAGES = [
  {
    id: 1,
    name: 'Francis Mwangi',
    phone: '0711223344',
    email: 'francis@example.com',
    message: 'Hi, do you have stock of MK double sockets in bulk? Need about 50 pieces for a site in Lurambi.',
    is_read: false,
    created_at: new Date(Date.now() - 3600000 * 4).toISOString()
  },
  {
    id: 2,
    name: 'Grace Chebet',
    phone: '0722998877',
    email: 'grace@example.com',
    message: 'Hello, what is the warranty period for your Solar King panels? Can I get delivery to Shikoti?',
    is_read: true,
    created_at: new Date(Date.now() - 3600000 * 18).toISOString()
  }
];

// Helper to initialize and get localStorage DB
function getLocalDB() {
  let db = localStorage.getItem('naoja_db');
  if (!db) {
    const initialDB = {
      categories: DEFAULT_CATEGORIES,
      products: DEFAULT_PRODUCTS,
      orders: DEFAULT_ORDERS,
      reviews: DEFAULT_REVIEWS,
      subscribers: DEFAULT_SUBSCRIBERS,
      messages: DEFAULT_MESSAGES
    };
    localStorage.setItem('naoja_db', JSON.stringify(initialDB));
    return initialDB;
  }
  try {
    return JSON.parse(db);
  } catch (e) {
    // If corruption, reset
    const initialDB = {
      categories: DEFAULT_CATEGORIES,
      products: DEFAULT_PRODUCTS,
      orders: DEFAULT_ORDERS,
      reviews: DEFAULT_REVIEWS,
      subscribers: DEFAULT_SUBSCRIBERS,
      messages: DEFAULT_MESSAGES
    };
    localStorage.setItem('naoja_db', JSON.stringify(initialDB));
    return initialDB;
  }
}

function saveLocalDB(db) {
  localStorage.setItem('naoja_db', JSON.stringify(db));
}

// Local Fallback Handler simulating backend router response structures
function handleLocalFallback(endpoint, options = {}) {
  const db = getLocalDB();
  const method = (options.method || 'GET').toUpperCase();
  const body = options.body ? (typeof options.body === 'string' ? JSON.parse(options.body) : options.body) : null;

  // 1. PRODUCTS ENDPOINTS
  if (endpoint.startsWith('/api/products')) {
    // Product details by slug
    if (method === 'GET' && !endpoint.includes('?')) {
      const parts = endpoint.split('/');
      if (parts.length > 3) {
        const slugOrId = parts[3];
        // check if slug
        let product = db.products.find(p => p.slug === slugOrId || p.id === parseInt(slugOrId));
        if (product) {
          // Increment views
          product.views = (product.views || 0) + 1;
          saveLocalDB(db);
          return product;
        }
        throw new Error('Product not found');
      }
    }

    // Admin get all or standard get products (with search/category queries)
    if (method === 'GET') {
      const urlObj = new URL(`http://localhost${endpoint}`);
      const categorySlug = urlObj.searchParams.get('category');
      const search = urlObj.searchParams.get('search');
      const isAdmin = urlObj.searchParams.get('admin') === 'true';

      let list = [...db.products];

      if (categorySlug) {
        const cat = db.categories.find(c => c.slug === categorySlug);
        if (cat) {
          list = list.filter(p => p.category_id === cat.id);
        } else {
          list = [];
        }
      }

      if (search) {
        const query = search.toLowerCase();
        list = list.filter(p => 
          p.name.toLowerCase().includes(query) || 
          p.description.toLowerCase().includes(query) ||
          (p.subcategory && p.subcategory.toLowerCase().includes(query))
        );
      }

      return list;
    }

    if (method === 'POST') {
      const newProduct = {
        id: db.products.length > 0 ? Math.max(...db.products.map(p => p.id)) + 1 : 1,
        name: body.name || 'Unnamed Product',
        slug: body.slug || (body.name ? body.name.toLowerCase().replace(/[^a-z0-9]+/g, '-') : 'new-product'),
        description: body.description || '',
        price: parseFloat(body.price || 0),
        stock_quantity: parseInt(body.stock_quantity || 0),
        category_id: parseInt(body.category_id || 1),
        subcategory: body.subcategory || '',
        images: body.images || ['https://images.unsplash.com/photo-1505740420928-5e560c06d30e?w=600'],
        status: body.stock_quantity <= 0 ? 'out_of_stock' : (body.stock_quantity <= 5 ? 'low_stock' : 'in_stock'),
        is_featured: !!body.is_featured,
        views: 0,
        rating_avg: 0.0,
        rating_count: 0
      };
      db.products.push(newProduct);
      saveLocalDB(db);
      return newProduct;
    }

    if (method === 'PUT') {
      const parts = endpoint.split('/');
      const id = parseInt(parts[3]);
      const idx = db.products.findIndex(p => p.id === id);
      if (idx !== -1) {
        const updated = {
          ...db.products[idx],
          ...body,
          price: body.price !== undefined ? parseFloat(body.price) : db.products[idx].price,
          stock_quantity: body.stock_quantity !== undefined ? parseInt(body.stock_quantity) : db.products[idx].stock_quantity,
          category_id: body.category_id !== undefined ? parseInt(body.category_id) : db.products[idx].category_id
        };
        // Auto status
        updated.status = updated.stock_quantity <= 0 ? 'out_of_stock' : (updated.stock_quantity <= 5 ? 'low_stock' : 'in_stock');
        db.products[idx] = updated;
        saveLocalDB(db);
        return updated;
      }
      throw new Error('Product not found for update');
    }

    if (method === 'DELETE') {
      const parts = endpoint.split('/');
      const id = parseInt(parts[3]);
      db.products = db.products.filter(p => p.id !== id);
      saveLocalDB(db);
      return { message: 'Product deleted successfully', id };
    }
  }

  // 2. CATEGORIES ENDPOINTS
  if (endpoint.startsWith('/api/categories')) {
    if (method === 'GET') {
      // Calculate dynamic counts based on current products
      return db.categories.map(cat => ({
        ...cat,
        product_count: db.products.filter(p => p.category_id === cat.id).length
      }));
    }

    if (method === 'POST') {
      const newCat = {
        id: db.categories.length > 0 ? Math.max(...db.categories.map(c => c.id)) + 1 : 1,
        name: body.name || 'Unnamed Category',
        slug: body.slug || (body.name ? body.name.toLowerCase().replace(/[^a-z0-9]+/g, '-') : 'new-cat'),
        icon: body.icon || '📁',
        product_count: 0
      };
      db.categories.push(newCat);
      saveLocalDB(db);
      return newCat;
    }

    if (method === 'PUT') {
      const parts = endpoint.split('/');
      const id = parseInt(parts[3]);
      const idx = db.categories.findIndex(c => c.id === id);
      if (idx !== -1) {
        db.categories[idx] = {
          ...db.categories[idx],
          ...body
        };
        saveLocalDB(db);
        return db.categories[idx];
      }
      throw new Error('Category not found for update');
    }

    if (method === 'DELETE') {
      const parts = endpoint.split('/');
      const id = parseInt(parts[3]);
      db.categories = db.categories.filter(c => c.id !== id);
      saveLocalDB(db);
      return { message: 'Category deleted successfully', id };
    }
  }

  // 3. ORDERS ENDPOINTS
  if (endpoint.startsWith('/api/orders')) {
    if (method === 'GET') {
      return db.orders;
    }

    if (method === 'POST') {
      // Create checkout order
      const newOrder = {
        id: db.orders.length > 0 ? Math.max(...db.orders.map(o => o.id)) + 1 : 1,
        order_ref: `NJ-${new Date().getFullYear()}-${String(db.orders.length + 1).padStart(4, '0')}`,
        customer_name: body.customer_name || 'Anonymous',
        customer_phone: body.customer_phone || '',
        customer_email: body.customer_email || '',
        total_amount: parseFloat(body.total_amount || 0),
        status: 'pending',
        payment_method: body.payment_method || 'mpesa',
        payment_status: 'pending',
        mpesa_till_number: '4149288',
        mpesa_transaction_id: body.mpesa_transaction_id || '',
        shipping_address: body.shipping_address || '',
        notes: body.notes || '',
        created_at: new Date().toISOString(),
        items: (body.items || []).map((item, index) => {
          // Decrement product stock in mock db
          const prodIdx = db.products.findIndex(p => p.id === item.product_id);
          if (prodIdx !== -1) {
            const currentStock = db.products[prodIdx].stock_quantity;
            db.products[prodIdx].stock_quantity = Math.max(0, currentStock - item.quantity);
            db.products[prodIdx].status = db.products[prodIdx].stock_quantity <= 0 
              ? 'out_of_stock' 
              : (db.products[prodIdx].stock_quantity <= 5 ? 'low_stock' : 'in_stock');
          }
          return {
            id: index + 1,
            product_id: item.product_id,
            product_name: item.product_name || 'Product',
            quantity: parseInt(item.quantity || 1),
            unit_price: parseFloat(item.unit_price || 0),
            total_price: parseFloat(item.unit_price || 0) * parseInt(item.quantity || 1)
          };
        })
      };
      db.orders.push(newOrder);
      saveLocalDB(db);
      return { message: 'Order placed successfully via LocalStorage Fallback', order: newOrder };
    }

    if (method === 'PUT') {
      const parts = endpoint.split('/');
      const id = parseInt(parts[3]);
      const idx = db.orders.findIndex(o => o.id === id);
      if (idx !== -1) {
        db.orders[idx] = {
          ...db.orders[idx],
          ...body,
          // Support setting both order status and payment status directly
          payment_status: body.payment_status || (body.status === 'delivered' ? 'paid' : db.orders[idx].payment_status)
        };
        saveLocalDB(db);
        return db.orders[idx];
      }
      throw new Error('Order not found for update');
    }
  }

  // 4. REVIEWS ENDPOINTS
  if (endpoint.startsWith('/api/reviews')) {
    if (method === 'GET') {
      const urlObj = new URL(`http://localhost${endpoint}`);
      const isAdmin = urlObj.searchParams.get('admin') === 'true';

      if (isAdmin) {
        return db.reviews;
      }
      // Return only approved reviews for homepage
      return db.reviews.filter(r => r.is_approved);
    }

    if (method === 'POST') {
      const newReview = {
        id: db.reviews.length > 0 ? Math.max(...db.reviews.map(r => r.id)) + 1 : 1,
        product_id: body.product_id || 1,
        name: body.name || 'Anonymous',
        rating: parseInt(body.rating || 5),
        comment: body.comment || '',
        is_approved: false, // requires admin approval
        created_at: new Date().toISOString()
      };
      db.reviews.push(newReview);
      saveLocalDB(db);
      return { message: 'Review submitted successfully! It will appear once approved.', review: newReview };
    }

    // Approve/Reject Review
    if (method === 'PUT') {
      const parts = endpoint.split('/');
      const id = parseInt(parts[3]);
      const idx = db.reviews.findIndex(r => r.id === id);
      if (idx !== -1) {
        if (endpoint.endsWith('/approve')) {
          db.reviews[idx].is_approved = true;
          // Calculate new average ratings for product
          const prodId = db.reviews[idx].product_id;
          const prodReviews = db.reviews.filter(r => r.product_id === prodId && r.is_approved);
          const prodIdx = db.products.findIndex(p => p.id === prodId);
          if (prodIdx !== -1 && prodReviews.length > 0) {
            const sum = prodReviews.reduce((acc, r) => acc + r.rating, 0);
            db.products[prodIdx].rating_avg = parseFloat((sum / prodReviews.length).toFixed(1));
            db.products[prodIdx].rating_count = prodReviews.length;
          }
        } else if (endpoint.endsWith('/reject')) {
          db.reviews[idx].is_approved = false;
        }
        saveLocalDB(db);
        return db.reviews[idx];
      }
      throw new Error('Review not found for update');
    }
  }

  // 5. NEWSLETTER SUBSCRIBERS
  if (endpoint.startsWith('/api/newsletter')) {
    if (method === 'GET') {
      return db.subscribers;
    }

    if (method === 'POST') {
      const email = body.email;
      if (db.subscribers.some(s => s.email === email)) {
        return { message: 'Email is already subscribed!' };
      }
      const newSub = {
        id: db.subscribers.length > 0 ? Math.max(...db.subscribers.map(s => s.id)) + 1 : 1,
        name: body.name || 'Subscriber',
        email: email,
        is_active: true,
        subscribed_at: new Date().toISOString()
      };
      db.subscribers.push(newSub);
      saveLocalDB(db);
      return { message: 'Subscribed to newsletter successfully!', subscriber: newSub };
    }
  }

  // 6. CONTACT MESSAGES
  if (endpoint.startsWith('/api/contacts')) {
    if (method === 'GET') {
      return db.messages;
    }

    if (method === 'POST') {
      const newMsg = {
        id: db.messages.length > 0 ? Math.max(...db.messages.map(m => m.id)) + 1 : 1,
        name: body.name || 'Anonymous',
        phone: body.phone || '',
        email: body.email || '',
        message: body.message || '',
        is_read: false,
        created_at: new Date().toISOString()
      };
      db.messages.push(newMsg);
      saveLocalDB(db);
      return { message: 'Message sent successfully! We will get in touch soon.', inquiry: newMsg };
    }
  }

  // Mock Upload Image
  if (endpoint.startsWith('/api/upload') && method === 'POST') {
    return { url: 'https://images.unsplash.com/photo-1609592424085-f509e51c8680?w=600' };
  }

  // 7. ANALYTICS ENDPOINT
  if (endpoint.startsWith('/api/analytics') && method === 'GET') {
    const totalSales = db.orders
      .filter(o => o.payment_status === 'paid' || o.status === 'delivered')
      .reduce((acc, o) => acc + o.total_amount, 0);

    const pendingMessagesCount = db.messages.filter(m => !m.is_read).length;
    const lowStockCount = db.products.filter(p => p.stock_quantity <= 5).length;

    // Monthly sales mock
    const monthlySales = [
      { name: 'Jan', sales: totalSales * 0.15 + 5000 },
      { name: 'Feb', sales: totalSales * 0.20 + 8000 },
      { name: 'Mar', sales: totalSales * 0.25 + 12000 },
      { name: 'Apr', sales: totalSales * 0.30 + 15000 },
      { name: 'May', sales: totalSales * 0.35 + 20000 },
      { name: 'Jun', sales: totalSales + 25000 }
    ];

    return {
      totalSales,
      totalOrders: db.orders.length,
      totalProducts: db.products.length,
      totalSubscribers: db.subscribers.length,
      lowStockCount,
      pendingMessagesCount,
      recentOrders: [...db.orders].sort((a,b) => new Date(b.created_at) - new Date(a.created_at)).slice(0, 5),
      monthlySales
    };
  }

  throw new Error(`Unsupported fallback endpoint: ${endpoint}`);
}

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

  try {
    const response = await fetch(url, config);
    const data = await response.json();

    if (!response.ok) {
      throw new Error(data.message || 'Something went wrong');
    }

    return data;
  } catch (error) {
    // Intercept network/connection errors and use localStorage fallback
    console.warn(`⚠️ API connection failed. Falling back to LocalStorage DB for [${options.method || 'GET'}] ${endpoint}`, error.message);
    
    // Artificial latency for smoother UI transitions
    await new Promise(resolve => setTimeout(resolve, 300));
    
    try {
      return handleLocalFallback(endpoint, options);
    } catch (fallbackError) {
      console.error(`❌ LocalStorage Fallback failed for endpoint: ${endpoint}`, fallbackError);
      throw fallbackError;
    }
  }
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
    updateOrderPaymentStatus: (id, paymentStatus) => request(`/api/orders/${id}`, { method: 'PUT', body: { payment_status: paymentStatus } }),

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
