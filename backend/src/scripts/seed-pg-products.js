require('dotenv').config({ path: './.env' });
const { pgPool } = require('../config/db');

const DEFAULT_PRODUCTS = [
  {
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
    name: 'Samsung Galaxy A35 5G',
    slug: 'samsung-galaxy-a35-5g',
    description: 'Samsung Galaxy A35 5G comes with a 6.6-inch Super AMOLED 120Hz display, a powerful 50MP triple-camera setup, Exynos 1380 processor, and a massive 5000mAh battery with 25W fast charging support.',
    price: 34500.00,
    stock_quantity: 2,
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
    name: 'Oraimo FreePods 4 Active Noise Cancelling',
    slug: 'oraimo-freepods-4-anc',
    description: 'Oraimo FreePods 4 features active noise cancellation, heavy bass, IPX5 waterproof design, up to 35.5-hour playtime with the charging case, and convenient touch controls. Connects via Bluetooth 5.2.',
    price: 4200.00,
    stock_quantity: 0,
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

async function seedProducts() {
  const client = await pgPool.connect();
  try {
    console.log('Seeding products into PostgreSQL...');
    for (const prod of DEFAULT_PRODUCTS) {
      await client.query(`
        INSERT INTO products (
          name, slug, description, price, stock_quantity,
          category_id, subcategory, images, status,
          is_featured, views, rating_avg, rating_count
        ) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13)
        ON CONFLICT (slug) DO UPDATE SET
          name = EXCLUDED.name,
          description = EXCLUDED.description,
          price = EXCLUDED.price,
          stock_quantity = EXCLUDED.stock_quantity,
          category_id = EXCLUDED.category_id,
          subcategory = EXCLUDED.subcategory,
          images = EXCLUDED.images,
          status = EXCLUDED.status,
          is_featured = EXCLUDED.is_featured;
      `, [
        prod.name,
        prod.slug,
        prod.description,
        prod.price,
        prod.stock_quantity,
        prod.category_id,
        prod.subcategory,
        prod.images,
        prod.status,
        prod.is_featured,
        prod.views,
        prod.rating_avg,
        prod.rating_count
      ]);
    }
    const { rows } = await client.query('SELECT count(*) FROM products');
    console.log(`✅ Successfully seeded products in PostgreSQL. Total products: ${rows[0].count}`);
  } finally {
    client.release();
    await pgPool.end();
  }
}

seedProducts().catch(err => {
  console.error('Failed to seed products:', err);
  process.exit(1);
});
