-- Insert categories
INSERT INTO categories (name, slug, description, product_count) VALUES
  ('Phones', 'phones', 'Smartphones & accessories', 45),
  ('Laptops & Computers', 'laptops-computers', 'Work & gaming laptops', 32),
  ('TVs & Displays', 'tvs-displays', '4K, smart TVs, monitors', 28),
  ('Audio', 'audio', 'Speakers & earphones', 56),
  ('Solar', 'solar', 'Panels, inverters, batteries', 24),
  ('Home Appliances', 'home-appliances', 'Kitchen & home appliances', 41),
  ('Electrical Installation', 'electrical-installation', 'Switches, breakers, cables', 89),
  ('Gaming Devices', 'gaming-devices', 'Consoles & controllers', 18),
  ('Accessories', 'accessories', 'Chargers, cables, cases', 124);

-- Insert products
INSERT INTO products (name, slug, description, price, category_name, brand, image_url, in_stock, highlights, specifications) VALUES
  ('Oraimo FreePods Pro', 'oraimo-freepods-pro', 'Premium wireless earbuds with active noise cancellation and crystal clear sound.', 3999, 'Audio', 'Oraimo', 'https://images.pexels.com/photos/3780670/pexels-photo-3780670.jpeg?auto=compress&cs=tinysrgb&w=400', true, ARRAY['Active Noise Cancellation', 'Bluetooth 5.2', '24hr Battery Life', 'Touch Controls'], '{"Driver Size": "10mm", "Battery": "24hr total", "Connectivity": "Bluetooth 5.2", "ANC": "Yes"}'::jsonb),
  
  ('Vitron 43" Smart TV', 'vitron-43-smart-tv', 'Full HD Smart TV with built-in streaming apps and stunning picture quality.', 18500, 'TVs & Displays', 'Vitron', 'https://images.pexels.com/photos/6937464/pexels-photo-6937464.jpeg?auto=compress&cs=tinysrgb&w=400', true, ARRAY['Full HD 1080p', 'Smart TV Apps', 'HDMI x2', 'USB Playback'], '{"Screen Size": "43 inch", "Resolution": "1080p Full HD", "Smart Apps": "Yes", "HDMI Ports": "2"}'::jsonb),
  
  ('JBL Go Speaker', 'jbl-go-speaker', 'Portable Bluetooth speaker with powerful sound and deep bass.', 4200, 'Audio', 'JBL', 'https://images.pexels.com/photos/1271911/pexels-photo-1271911.jpeg?auto=compress&cs=tinysrgb&w=400', true, ARRAY['5W Output', 'Deep Bass', 'Compact Design', '5hr Battery'], '{"Output": "5W", "Connectivity": "Bluetooth", "Battery": "5 hours", "Ports": "USB-C"}'::jsonb),
  
  ('Laptop (16GB/512GB)', 'laptop-16gb-512gb', 'Powerful laptop for work and study with fast processor and ample storage.', 55000, 'Laptops & Computers', 'Lenovo', 'https://images.pexels.com/photos/18105731/pexels-photo-18105731.jpeg?auto=compress&cs=tinysrgb&w=400', true, ARRAY['16GB RAM', '512GB SSD', 'Fast Processor', '15.6 inch Display'], '{"RAM": "16GB", "Storage": "512GB SSD", "Display": "15.6 inch", "Processor": "Intel Core i5"}'::jsonb),
  
  ('Samsung Galaxy A14', 'samsung-galaxy-a14', 'Affordable smartphone with 128GB storage and dual SIM support.', 24999, 'Phones', 'Samsung', 'https://images.pexels.com/photos/607812/pexels-photo-607812.jpeg?auto=compress&cs=tinysrgb&w=400', true, ARRAY['128GB Storage', 'Dual SIM', '6.6 inch Display', '50MP Camera'], '{"Storage": "128GB", "Display": "6.6 inch", "Camera": "50MP", "Battery": "5000mAh"}'::jsonb),
  
  ('Oraimo Charger Kit', 'oraimo-charger-kit', 'Type-C fast charging adapter with cable.', 1200, 'Accessories', 'Oraimo', 'https://images.pexels.com/photos/1631005/pexels-photo-1631005.jpeg?auto=compress&cs=tinysrgb&w=400', true, ARRAY['20W Fast Charging', 'Type-C', 'Compact Design'], '{"Output": "20W", "Type": "Type-C", "Cable Length": "1m"}'::jsonb),
  
  ('HP Laptop 15s', 'hp-laptop-15s', 'Budget-friendly laptop with 8GB RAM and 256GB SSD.', 42000, 'Laptops & Computers', 'HP', 'https://images.pexels.com/photos/18105731/pexels-photo-18105731.jpeg?auto=compress&cs=tinysrgb&w=400', true, ARRAY['8GB RAM', '256GB SSD', '15.6 inch HD Display'], '{"RAM": "8GB", "Storage": "256GB SSD", "Display": "15.6 inch HD"}'::jsonb),
  
  ('Solar Panel 100W', 'solar-panel-100w', 'High-efficiency monocrystalline solar panel for home use.', 8500, 'Solar', 'Generic', 'https://images.pexels.com/photos/9873824/pexels-photo-9873824.jpeg?auto=compress&cs=tinysrgb&w=400', true, ARRAY['100W Output', 'Monocrystalline', 'Durable Frame'], '{"Power": "100W", "Type": "Monocrystalline", "Dimensions": "120x60cm"}'::jsonb),

  ('Samsung 55" 4K TV', 'samsung-55-4k-tv', 'Crystal UHD Smart TV with stunning 4K resolution.', 65000, 'TVs & Displays', 'Samsung', 'https://images.pexels.com/photos/4006105/pexels-photo-4006105.jpeg?auto=compress&cs=tinysrgb&w=400', true, ARRAY['4K Crystal UHD', 'Smart TV', 'HDR', 'ThinQ AI'], '{"Screen Size": "55 inch", "Resolution": "4K UHD", "HDR": "Yes", "Smart Apps": "Tizen OS"}'::jsonb),

  ('Phone Case Premium', 'phone-case-premium', 'Shockproof protective phone case with premium finish.', 800, 'Accessories', 'Generic', 'https://images.pexels.com/photos/1434237/pexels-photo-1434237.jpeg?auto=compress&cs=tinysrgb&w=400', true, ARRAY['Shockproof', 'Premium Finish', 'Perfect Fit'], '{"Material": "TPU + PC", "Protection": "Military Grade"}'::jsonb),

  ('USB-C Cable 1m', 'usb-c-cable-1m', 'Fast charging USB-C cable, 1 meter length.', 350, 'Accessories', 'Oraimo', 'https://images.pexels.com/photos/1631005/pexels-photo-1631005.jpeg?auto=compress&cs=tinysrgb&w=400', true, ARRAY['Fast Charging', 'Data Transfer', 'Durable'], '{"Length": "1m", "Type": "USB-C to USB-C", "Current": "3A"}'::jsonb);
