import { Link } from 'react-router-dom';
import { useState, useEffect } from 'react';
import Layout from '../components/Layout';
import ProductCard from '../components/ProductCard';
import CategoryCard from '../components/CategoryCard';
import { supabase } from '../lib/supabase';
import { Product, Category } from '../types';

const DEFAULT_CATEGORIES: Category[] = [
  { id: '1', name: 'Phones', slug: 'phones', description: 'Smartphones & accessories', image_url: '', product_count: 45 },
  { id: '2', name: 'Laptops & Computers', slug: 'laptops-computers', description: 'Work & gaming', image_url: '', product_count: 32 },
  { id: '3', name: 'TVs & Displays', slug: 'tvs-displays', description: '4K, smart TVs', image_url: '', product_count: 28 },
  { id: '4', name: 'Audio', slug: 'audio', description: 'Speakers & earphones', image_url: '', product_count: 56 },
  { id: '5', name: 'Solar', slug: 'solar', description: 'Panels, inverters', image_url: '', product_count: 24 },
  { id: '6', name: 'Home Appliances', slug: 'home-appliances', description: 'Kitchen & home', image_url: '', product_count: 41 },
  { id: '7', name: 'Electrical Installation', slug: 'electrical-installation', description: 'Switches, breakers', image_url: '', product_count: 89 },
  { id: '8', name: 'Gaming Devices', slug: 'gaming-devices', description: 'Consoles & pads', image_url: '', product_count: 18 },
  { id: '9', name: 'Accessories', slug: 'accessories', description: 'Chargers, cables', image_url: '', product_count: 124 },
];

const DEFAULT_PRODUCTS: Product[] = [
  {
    id: '1',
    name: 'Oraimo FreePods Pro',
    slug: 'oraimo-freepods-pro',
    description: 'Premium wireless earbuds with active noise cancellation',
    price: 3999,
    category_id: '4',
    category_name: 'Audio',
    brand: 'Oraimo',
    image_url: 'https://images.pexels.com/photos/3780670/pexels-photo-3780670.jpeg?auto=compress&cs=tinysrgb&w=400',
    in_stock: true,
    highlights: ['Bluetooth 5.2', 'Active Noise Cancellation', '24hr Battery Life'],
    specifications: { 'Driver Size': '10mm', 'Battery': '24hr total', 'Connectivity': 'Bluetooth 5.2' },
    created_at: new Date().toISOString(),
  },
  {
    id: '2',
    name: 'Vitron 43" Smart TV',
    slug: 'vitron-43-smart-tv',
    description: 'Full HD Smart TV with built-in apps',
    price: 18500,
    category_id: '3',
    category_name: 'TVs & Displays',
    brand: 'Vitron',
    image_url: 'https://images.pexels.com/photos/6937464/pexels-photo-6937464.jpeg?auto=compress&cs=tinysrgb&w=400',
    in_stock: true,
    highlights: ['Full HD 1080p', 'Smart TV Apps', 'HDMI x2'],
    specifications: { 'Screen Size': '43"', 'Resolution': '1080p Full HD', 'Smart Apps': 'Yes' },
    created_at: new Date().toISOString(),
  },
  {
    id: '3',
    name: 'JBL Go Speaker',
    slug: 'jbl-go-speaker',
    description: 'Portable Bluetooth speaker with powerful sound',
    price: 4200,
    category_id: '4',
    category_name: 'Audio',
    brand: 'JBL',
    image_url: 'https://images.pexels.com/photos/1271911/pexels-photo-1271911.jpeg?auto=compress&cs=tinysrgb&w=400',
    in_stock: true,
    highlights: ['5W Output', 'Deep Bass', 'Compact Design'],
    specifications: { 'Output': '5W', 'Connectivity': 'Bluetooth', 'Battery': '5hr' },
    created_at: new Date().toISOString(),
  },
  {
    id: '4',
    name: 'Laptop (16GB/512GB)',
    slug: 'laptop-16gb-512gb',
    description: 'Powerful laptop for work and study',
    price: 55000,
    category_id: '2',
    category_name: 'Laptops & Computers',
    brand: 'Lenovo',
    image_url: 'https://images.pexels.com/photos/18105731/pexels-photo-18105731.jpeg?auto=compress&cs=tinysrgb&w=400',
    in_stock: true,
    highlights: ['16GB RAM', '512GB SSD', 'Fast Processor'],
    specifications: { 'RAM': '16GB', 'Storage': '512GB SSD', 'Display': '15.6"' },
    created_at: new Date().toISOString(),
  },
];

export default function HomePage() {
  const [categories, setCategories] = useState<Category[]>(DEFAULT_CATEGORIES);
  const [products, setProducts] = useState<Product[]>(DEFAULT_PRODUCTS);

  useEffect(() => {
    async function fetchData() {
      try {
        const [catRes, prodRes] = await Promise.all([
          supabase.from('categories').select('*').order('name'),
          supabase.from('products').select('*').limit(8).order('created_at', { ascending: false }),
        ]);

        if (catRes.data && catRes.data.length > 0) {
          setCategories(catRes.data);
        }
        if (prodRes.data && prodRes.data.length > 0) {
          setProducts(prodRes.data);
        }
      } catch (error) {
        console.error('Error fetching data:', error);
      }
    }

    fetchData();
  }, []);

  return (
    <Layout>
      {/* Hero Section */}
      <section className="py-16">
        <div className="max-w-7xl mx-auto px-4">
          <div className="grid lg:grid-cols-5 gap-6 items-center">
            <div className="lg:col-span-3">
              <h1 className="font-serif text-4xl lg:text-5xl leading-tight text-gray-900 mb-3">
                Premium Electronics & Solar Solutions
              </h1>
              <p className="text-gray-500 text-base max-w-xl">
                Pay with M-Pesa - Same-day delivery before 4pm (Sun-Fri) - Pickup ready in 1-2 hours
              </p>
              <div className="flex flex-wrap gap-3 mt-5">
                <Link to="/category/phones" className="btn-primary">
                  Shop Best Sellers
                </Link>
                <Link to="/account?tab=location" className="btn-secondary">
                  Visit Store Location
                </Link>
              </div>
              <div className="flex flex-wrap gap-2 mt-5">
                <span className="bg-surface border border-gray-200 rounded-full px-3 py-1.5 text-xs text-gray-500">
                  Free delivery &ge; <strong className="text-gray-900">KSh 600</strong>
                </span>
                <span className="bg-surface border border-gray-200 rounded-full px-3 py-1.5 text-xs text-gray-500">
                  Pickup: Kakamega (Lurambi)
                </span>
                <span className="bg-surface border border-gray-200 rounded-full px-3 py-1.5 text-xs text-gray-500">
                  Closed Saturdays
                </span>
              </div>
            </div>

            <div className="lg:col-span-2 h-80 rounded-2xl border border-gray-200 bg-gradient-to-br from-brand-100/50 to-transparent" />
          </div>
        </div>
      </section>

      {/* Categories */}
      <section className="py-8">
        <div className="max-w-7xl mx-auto px-4">
          <h2 className="font-serif text-3xl text-gray-900 mb-6">Shop by Category</h2>
          <div className="grid grid-cols-1 sm:grid-cols-2 lg:grid-cols-3 gap-6">
            {categories.slice(0, 9).map((cat) => (
              <CategoryCard key={cat.id} category={cat} />
            ))}
          </div>
        </div>
      </section>

      {/* Best Sellers */}
      <section className="py-8">
        <div className="max-w-7xl mx-auto px-4">
          <h2 className="font-serif text-3xl text-gray-900 mb-6">Best Sellers</h2>
          <div className="grid grid-cols-1 sm:grid-cols-2 lg:grid-cols-4 gap-6">
            {products.map((product) => (
              <ProductCard key={product.id} product={product} />
            ))}
          </div>

          {/* Why Naoja */}
          <div className="mt-8 bg-surface border border-gray-200 rounded-card p-6">
            <div className="grid md:grid-cols-2 gap-6">
              <div>
                <h3 className="font-serif text-2xl text-gray-900 mb-2">Why Naoja</h3>
                <p className="text-gray-500 text-sm">
                  Genuine products, secure M-Pesa checkout, and fast Kakamega fulfillment.
                </p>
              </div>
              <div className="space-y-2.5">
                <div className="bg-white border border-gray-200 rounded-card p-3.5 text-sm">
                  Genuine products - Warranty support
                </div>
                <div className="bg-white border border-gray-200 rounded-card p-3.5 text-sm">
                  Delivery Sun-Fri - Order before 4pm
                </div>
                <div className="bg-white border border-gray-200 rounded-card p-3.5 text-sm">
                  Pickup ready in 1-2 hrs - Closed Saturdays
                </div>
              </div>
            </div>
          </div>

          {/* Brands */}
          <div className="mt-6">
            <p className="text-gray-500 text-sm mb-2.5">Brands</p>
            <div className="bg-white border border-gray-200 rounded-card p-4 flex flex-wrap gap-4">
              {['Apple', 'Samsung', 'HP', 'Dell', 'Lenovo', 'Vitron', 'JBL', 'Oraimo'].map((brand) => (
                <span key={brand} className="text-gray-500 text-sm">{brand}</span>
              ))}
            </div>
          </div>
        </div>
      </section>
    </Layout>
  );
}
