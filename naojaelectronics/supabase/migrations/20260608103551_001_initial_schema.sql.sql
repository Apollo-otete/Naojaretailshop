-- Categories table
CREATE TABLE categories (
  id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  name TEXT NOT NULL,
  slug TEXT UNIQUE NOT NULL,
  description TEXT,
  image_url TEXT,
  product_count INTEGER DEFAULT 0,
  created_at TIMESTAMPTZ DEFAULT NOW()
);

-- Products table
CREATE TABLE products (
  id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  name TEXT NOT NULL,
  slug TEXT UNIQUE NOT NULL,
  description TEXT,
  price INTEGER NOT NULL,
  category_id UUID REFERENCES categories(id) ON DELETE SET NULL,
  category_name TEXT,
  brand TEXT,
  image_url TEXT,
  in_stock BOOLEAN DEFAULT true,
  highlights TEXT[],
  specifications JSONB DEFAULT '{}',
  created_at TIMESTAMPTZ DEFAULT NOW()
);

-- Orders table
CREATE TABLE orders (
  id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  customer_name TEXT NOT NULL,
  customer_phone TEXT NOT NULL,
  fulfillment_type TEXT NOT NULL CHECK (fulfillment_type IN ('delivery', 'pickup')),
  delivery_area TEXT,
  delivery_directions TEXT,
  status TEXT DEFAULT 'pending' CHECK (status IN ('pending', 'confirmed', 'processing', 'delivered', 'cancelled')),
  subtotal INTEGER NOT NULL,
  delivery_fee INTEGER DEFAULT 0,
  total INTEGER NOT NULL,
  created_at TIMESTAMPTZ DEFAULT NOW()
);

-- Order items table
CREATE TABLE order_items (
  id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  order_id UUID REFERENCES orders(id) ON DELETE CASCADE,
  product_id UUID REFERENCES products(id) ON DELETE SET NULL,
  product_name TEXT NOT NULL,
  quantity INTEGER NOT NULL,
  price INTEGER NOT NULL
);

-- Enable RLS
ALTER TABLE categories ENABLE ROW LEVEL SECURITY;
ALTER TABLE products ENABLE ROW LEVEL SECURITY;
ALTER TABLE orders ENABLE ROW LEVEL SECURITY;
ALTER TABLE order_items ENABLE ROW LEVEL SECURITY;

-- RLS Policies for categories (public read)
CREATE POLICY "categories_public_read" ON categories FOR SELECT TO anon, authenticated USING (true);

-- RLS Policies for products (public read)
CREATE POLICY "products_public_read" ON products FOR SELECT TO anon, authenticated USING (true);

-- RLS Policies for orders (authenticated only)
CREATE POLICY "orders_authenticated_all" ON orders FOR ALL TO authenticated USING (true) WITH CHECK (true);

-- RLS Policies for order_items (authenticated only)
CREATE POLICY "order_items_authenticated_all" ON order_items FOR ALL TO authenticated USING (true) WITH CHECK (true);

-- Create indexes for better query performance
CREATE INDEX idx_products_category ON products(category_id);
CREATE INDEX idx_products_slug ON products(slug);
CREATE INDEX idx_categories_slug ON categories(slug);
CREATE INDEX idx_orders_created ON orders(created_at);
