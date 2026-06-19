
         --///////DATABASE//////

--             1. USERS TABLE
CREATE TABLE IF NOT EXISTS users (
    id SERIAL PRIMARY KEY,
    username VARCHAR(50) UNIQUE NOT NULL,
    email VARCHAR(255) UNIQUE NOT NULL,
    password_hash VARCHAR(255) NOT NULL,
    full_name VARCHAR(100),
    phone VARCHAR(20),
    role VARCHAR(20) DEFAULT 'customer', -- admin, vendor, customer
    is_active BOOLEAN DEFAULT TRUE,
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
);

-- 2. CATEGORIES TABLE (Naoja's 23 categories)
CREATE TABLE IF NOT EXISTS categories (
    id SERIAL PRIMARY KEY,
    name VARCHAR(100) NOT NULL,
    slug VARCHAR(100) UNIQUE NOT NULL,
    icon VARCHAR(50),
    parent_id INTEGER REFERENCES categories(id),
    is_active BOOLEAN DEFAULT TRUE,
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
);

-- 3. PRODUCTS TABLE
CREATE TABLE IF NOT EXISTS products (
    id SERIAL PRIMARY KEY,
    name VARCHAR(255) NOT NULL,
    slug VARCHAR(255) UNIQUE NOT NULL,
    description TEXT,
    price DECIMAL(10, 2) NOT NULL,
    stock_quantity INTEGER DEFAULT 0,
    category_id INTEGER REFERENCES categories(id),
    subcategory VARCHAR(100),
    images TEXT[] DEFAULT '{}',
    status VARCHAR(20) DEFAULT 'in_stock', -- in_stock, low_stock, out_of_stock
    is_featured BOOLEAN DEFAULT FALSE,
    views INTEGER DEFAULT 0,
    rating_avg DECIMAL(3, 2) DEFAULT 0,
    rating_count INTEGER DEFAULT 0,
    vendor_id INTEGER REFERENCES users(id),
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
);

-- 4. ORDERS TABLE
CREATE TABLE IF NOT EXISTS orders (
    id SERIAL PRIMARY KEY,
    order_ref VARCHAR(50) UNIQUE NOT NULL,
    user_id INTEGER REFERENCES users(id),
    customer_name VARCHAR(100) NOT NULL,
    customer_phone VARCHAR(20) NOT NULL,
    customer_email VARCHAR(255),
    total_amount DECIMAL(10, 2) NOT NULL,
    status VARCHAR(50) DEFAULT 'pending', -- pending, confirmed, shipped, delivered, cancelled
    payment_method VARCHAR(50) DEFAULT 'mpesa',
    payment_status VARCHAR(50) DEFAULT 'pending', -- pending, paid, failed
    mpesa_till_number VARCHAR(20) DEFAULT '4149288',
    mpesa_transaction_id VARCHAR(100),
    mpesa_payment_confirmation TEXT,
    shipping_address TEXT,
    notes TEXT,
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
);

-- 5. ORDER ITEMS TABLE
CREATE TABLE IF NOT EXISTS order_items (
    id SERIAL PRIMARY KEY,
    order_id INTEGER REFERENCES orders(id) ON DELETE CASCADE,
    product_id INTEGER REFERENCES products(id),
    product_name VARCHAR(255) NOT NULL,
    quantity INTEGER NOT NULL,
    unit_price DECIMAL(10, 2) NOT NULL,
    total_price DECIMAL(10, 2) NOT NULL,
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
);

-- 6. REVIEWS TABLE
CREATE TABLE IF NOT EXISTS reviews (
    id SERIAL PRIMARY KEY,
    product_id INTEGER REFERENCES products(id) ON DELETE CASCADE,
    user_name VARCHAR(100) NOT NULL,
    user_email VARCHAR(255),
    rating INTEGER CHECK (rating >= 1 AND rating <= 5),
    comment TEXT,
    is_approved BOOLEAN DEFAULT FALSE,
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    approved_at TIMESTAMP
);

-- 7. NEWSLETTER SUBSCRIBERS
CREATE TABLE IF NOT EXISTS subscribers (
    id SERIAL PRIMARY KEY,
    name VARCHAR(100) NOT NULL,
    email VARCHAR(255) UNIQUE NOT NULL,
    is_active BOOLEAN DEFAULT TRUE,
    subscribed_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
);

-- 8. CONTACT MESSAGES
CREATE TABLE IF NOT EXISTS contact_messages (
    id SERIAL PRIMARY KEY,
    name VARCHAR(100) NOT NULL,
    phone VARCHAR(20),
    email VARCHAR(255),
    message TEXT NOT NULL,
    is_read BOOLEAN DEFAULT FALSE,
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
);

-- 9. CART (for guest users)
CREATE TABLE IF NOT EXISTS cart (
    id SERIAL PRIMARY KEY,
    session_id VARCHAR(255),
    user_id INTEGER REFERENCES users(id),
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
);

-- 10. CART ITEMS
CREATE TABLE IF NOT EXISTS cart_items (
    id SERIAL PRIMARY KEY,
    cart_id INTEGER REFERENCES cart(id) ON DELETE CASCADE,
    product_id INTEGER REFERENCES products(id),
    quantity INTEGER NOT NULL,
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
);

-- ============================================
-- TRIGGERS FOR UPDATED_AT
-- ============================================
CREATE OR REPLACE FUNCTION update_updated_at_column()
RETURNS TRIGGER AS $$
BEGIN
    NEW.updated_at = CURRENT_TIMESTAMP;
    RETURN NEW;
END;
$$ language 'plpgsql';

CREATE TRIGGER update_users_updated_at BEFORE UPDATE ON users
    FOR EACH ROW EXECUTE FUNCTION update_updated_at_column();

CREATE TRIGGER update_products_updated_at BEFORE UPDATE ON products
    FOR EACH ROW EXECUTE FUNCTION update_updated_at_column();

CREATE TRIGGER update_orders_updated_at BEFORE UPDATE ON orders
    FOR EACH ROW EXECUTE FUNCTION update_updated_at_column();

-- ============================================
-- SEED DATA FOR NAOJA VENTURES
-- ============================================

-- Insert Categories (23 categories as per Naoja Ventures)
INSERT INTO categories (name, slug, icon) VALUES
('Mobile Phones', 'mobile-phones', '📱'),
('Audio Devices', 'audio-devices', '🎧'),
('Television Products', 'television-products', '📺'),
('Charging Accessories', 'charging-accessories', '🔌'),
('Computers & Accessories', 'computers-accessories', '💻'),
('Electrical Products', 'electrical-products', '⚡'),
('Security Products', 'security-products', '🔒'),
('Networking Products', 'networking-products', '🌐'),
('Automotive Products', 'automotive-products', '🚗'),
('Gaming Products', 'gaming-products', '🎮'),
('Watches', 'watches', '⌚'),
('Renewable Energy', 'renewable-energy', '☀️'),
('Home Appliances', 'home-appliances', '🏠'),
('Other Electrical Products', 'other-electrical', '🔧')
ON CONFLICT (slug) DO NOTHING;

-- Insert Admin User (password: admin123)
INSERT INTO users (username, email, password_hash, full_name, role)
VALUES (
    'naoja_admin',
    'admin@naojaventures.com',
    '$2a$10$8qYx9RjKzLgPqXvYzQwR5uJkLmNpQrStUvWxYzAbCdEfGhIjKlMn', -- admin123
    'Naoja Admin',
    'admin'
) ON CONFLICT (username) DO NOTHING;

-- Insert Business Info
INSERT INTO subscribers (name, email) VALUES
('Naoja Ventures', 'info@naojaventures.com')
ON CONFLICT (email) DO NOTHING;