import { useState, useEffect } from 'react';
import { useParams, Link } from 'react-router-dom';
import Layout from '../components/Layout';
import ProductCard from '../components/ProductCard';
import { supabase } from '../lib/supabase';
import { Product, Category } from '../types';

const CATEGORY_DATA: Record<string, Product[]> = {
  phones: [
    { id: 'p1', name: 'Samsung Galaxy A14', slug: 'samsung-galaxy-a14', description: '128GB Dual SIM', price: 24999, category_id: '1', category_name: 'Phones', brand: 'Samsung', image_url: 'https://images.pexels.com/photos/607812/pexels-photo-607812.jpeg?auto=compress&cs=tinysrgb&w=400', in_stock: true, highlights: [], specifications: {}, created_at: '' },
    { id: 'p2', name: 'iPhone 13', slug: 'iphone-13', description: '128GB', price: 85000, category_id: '1', category_name: 'Phones', brand: 'Apple', image_url: 'https://images.pexels.com/photos/5754090/pexels-photo-5754090.jpeg?auto=compress&cs=tinysrgb&w=400', in_stock: true, highlights: [], specifications: {}, created_at: '' },
    { id: 'p3', name: 'Oraimo Charger Kit', slug: 'oraimo-charger-kit', description: 'Type-C 20W', price: 1200, category_id: '1', category_name: 'Phones', brand: 'Oraimo', image_url: 'https://images.pexels.com/photos/1631005/pexels-photo-1631005.jpeg?auto=compress&cs=tinysrgb&w=400', in_stock: true, highlights: [], specifications: {}, created_at: '' },
    { id: 'p4', name: 'Phone Case Premium', slug: 'phone-case-premium', description: 'Shockproof', price: 800, category_id: '1', category_name: 'Phones', brand: 'Generic', image_url: 'https://images.pexels.com/photos/1434237/pexels-photo-1434237.jpeg?auto=compress&cs=tinysrgb&w=400', in_stock: true, highlights: [], specifications: {}, created_at: '' },
  ],
  audio: [
    { id: 'a1', name: 'Oraimo FreePods Pro', slug: 'oraimo-freepods-pro', description: 'Premium wireless earbuds', price: 3999, category_id: '4', category_name: 'Audio', brand: 'Oraimo', image_url: 'https://images.pexels.com/photos/3780670/pexels-photo-3780670.jpeg?auto=compress&cs=tinysrgb&w=400', in_stock: true, highlights: [], specifications: {}, created_at: '' },
    { id: 'a2', name: 'JBL Go Speaker', slug: 'jbl-go-speaker', description: 'Portable Bluetooth', price: 4200, category_id: '4', category_name: 'Audio', brand: 'JBL', image_url: 'https://images.pexels.com/photos/1271911/pexels-photo-1271911.jpeg?auto=compress&cs=tinysrgb&w=400', in_stock: true, highlights: [], specifications: {}, created_at: '' },
    { id: 'a3', name: 'Oraim SoundBite', slug: 'oraim-soundbite', description: 'Mini speaker', price: 1500, category_id: '4', category_name: 'Audio', brand: 'Oraimo', image_url: 'https://images.pexels.com/photos/1649771/pexels-photo-1649771.jpeg?auto=compress&cs=tinysrgb&w=400', in_stock: true, highlights: [], specifications: {}, created_at: '' },
  ],
  'tvs-displays': [
    { id: 't1', name: 'Vitron 43" Smart TV', slug: 'vitron-43-smart-tv', description: 'Full HD Smart TV', price: 18500, category_id: '3', category_name: 'TVs & Displays', brand: 'Vitron', image_url: 'https://images.pexels.com/photos/6937464/pexels-photo-6937464.jpeg?auto=compress&cs=tinysrgb&w=400', in_stock: true, highlights: [], specifications: {}, created_at: '' },
    { id: 't2', name: 'Samsung 55" 4K TV', slug: 'samsung-55-4k-tv', description: 'Crystal UHD', price: 65000, category_id: '3', category_name: 'TVs & Displays', brand: 'Samsung', image_url: 'https://images.pexels.com/photos/4006105/pexels-photo-4006105.jpeg?auto=compress&cs=tinysrgb&w=400', in_stock: true, highlights: [], specifications: {}, created_at: '' },
  ],
  'laptops-computers': [
    { id: 'l1', name: 'HP Laptop 15s', slug: 'hp-laptop-15s', description: '8GB/256GB', price: 42000, category_id: '2', category_name: 'Laptops & Computers', brand: 'HP', image_url: 'https://images.pexels.com/photos/18105731/pexels-photo-18105731.jpeg?auto=compress&cs=tinysrgb&w=400', in_stock: true, highlights: [], specifications: {}, created_at: '' },
    { id: 'l2', name: 'Lenovo IdeaPad', slug: 'lenovo-ideapad', description: '16GB/512GB', price: 55000, category_id: '2', category_name: 'Laptops & Computers', brand: 'Lenovo', image_url: 'https://images.pexels.com/photos/18105731/pexels-photo-18105731.jpeg?auto=compress&cs=tinysrgb&w=400', in_stock: true, highlights: [], specifications: {}, created_at: '' },
  ],
  solar: [
    { id: 's1', name: 'Solar Panel 100W', slug: 'solar-panel-100w', description: 'Monocrystalline', price: 8500, category_id: '5', category_name: 'Solar', brand: 'Generic', image_url: 'https://images.pexels.com/photos/9873824/pexels-photo-9873824.jpeg?auto=compress&cs=tinysrgb&w=400', in_stock: true, highlights: [], specifications: {}, created_at: '' },
    { id: 's2', name: 'Solar Inverter 1kW', slug: 'solar-inverter-1kw', description: 'Pure Sine Wave', price: 25000, category_id: '5', category_name: 'Solar', brand: 'Generic', image_url: 'https://images.pexels.com/photos/9873824/pexels-photo-9873824.jpeg?auto=compress&cs=tinysrgb&w=400', in_stock: true, highlights: [], specifications: {}, created_at: '' },
  ],
  'home-appliances': [
    { id: 'h1', name: 'Electric Kettle', slug: 'electric-kettle', description: '1.7L', price: 2500, category_id: '6', category_name: 'Home Appliances', brand: 'Ramtons', image_url: 'https://images.pexels.com/photos/5738533/pexels-photo-5738533.jpeg?auto=compress&cs=tinysrgb&w=400', in_stock: true, highlights: [], specifications: {}, created_at: '' },
  ],
  'electrical-installation': [
    { id: 'e1', name: 'Circuit Breaker 32A', slug: 'circuit-breaker-32a', description: 'Single Pole', price: 450, category_id: '7', category_name: 'Electrical Installation', brand: 'Schneider', image_url: 'https://images.pexels.com/photos/2581929/pexels-photo-2581929.jpeg?auto=compress&cs=tinysrgb&w=400', in_stock: true, highlights: [], specifications: {}, created_at: '' },
  ],
  'gaming-devices': [
    { id: 'g1', name: 'PlayStation 5 Controller', slug: 'ps5-controller', description: 'DualSense', price: 9500, category_id: '8', category_name: 'Gaming Devices', brand: 'Sony', image_url: 'https://images.pexels.com/photos/3292566/pexels-photo-3292566.jpeg?auto=compress&cs=tinysrgb&w=400', in_stock: true, highlights: [], specifications: {}, created_at: '' },
  ],
  accessories: [
    { id: 'x1', name: 'USB-C Cable 1m', slug: 'usb-c-cable-1m', description: 'Fast charging', price: 350, category_id: '9', category_name: 'Accessories', brand: 'Oraimo', image_url: 'https://images.pexels.com/photos/1631005/pexels-photo-1631005.jpeg?auto=compress&cs=tinysrgb&w=400', in_stock: true, highlights: [], specifications: {}, created_at: '' },
  ],
};

const CATEGORY_NAMES: Record<string, string> = {
  phones: 'Phones',
  'laptops-computers': 'Laptops & Computers',
  'tvs-displays': 'TVs & Displays',
  audio: 'Audio',
  solar: 'Solar',
  'home-appliances': 'Home Appliances',
  'electrical-installation': 'Electrical Installation Accessories',
  'gaming-devices': 'Gaming Devices',
  accessories: 'Accessories',
};

export default function CategoryPage() {
  const { slug } = useParams();
  const [products, setProducts] = useState<Product[]>([]);
  const [category, setCategory] = useState<Category | null>(null);
  const [loading, setLoading] = useState(true);
  const [filters, setFilters] = useState({
    brand: [] as string[],
    priceRange: '',
    inStock: false,
  });

  useEffect(() => {
    async function fetchCategory() {
      setLoading(true);
      try {
        const { data: catData } = await supabase
          .from('categories')
          .select('*')
          .eq('slug', slug)
          .single();

        if (catData) {
          setCategory(catData);
          const { data: prodData } = await supabase
            .from('products')
            .select('*')
            .eq('category_id', catData.id);
          if (prodData && prodData.length > 0) {
            setProducts(prodData);
          } else {
            setProducts(CATEGORY_DATA[slug || ''] || []);
          }
        } else {
          setProducts(CATEGORY_DATA[slug || ''] || []);
        }
      } catch {
        setProducts(CATEGORY_DATA[slug || ''] || []);
      } finally {
        setLoading(false);
      }
    }

    fetchCategory();
  }, [slug]);

  const categoryTitle = CATEGORY_NAMES[slug || ''] || category?.name || 'Category';

  return (
    <Layout>
      <div className="max-w-7xl mx-auto px-4">
        {/* Breadcrumb */}
        <div className="text-xs text-gray-500 py-4">
          <Link to="/">Home</Link>
          <span className="mx-2">/</span>
          <span className="text-gray-900">{categoryTitle}</span>
        </div>

        {/* Header */}
        <div className="flex flex-wrap justify-between items-end gap-4 mb-6">
          <div>
            <h1 className="font-serif text-3xl text-gray-900">{categoryTitle}</h1>
            <p className="text-xs text-gray-500 mt-1">
              {products.length} results - Free delivery from KSh 600 - Pickup ready in 1-2 hrs
            </p>
          </div>
          <div className="bg-white border border-gray-200 rounded-card px-3 py-2.5 text-xs">
            <span className="text-gray-500">Sort:</span>
            <strong className="ml-1.5 text-gray-900">Recommended</strong>
          </div>
        </div>

        <div className="grid lg:grid-cols-[280px_1fr] gap-6 pb-8">
          {/* Filters */}
          <aside className="hidden lg:block bg-white border border-gray-200 rounded-card p-4">
            <h4 className="font-bold text-gray-900 mb-4">Filters</h4>

            <div className="mb-4">
              <p className="text-xs text-gray-500 mb-1.5">Brand</p>
              {['Samsung', 'Apple', 'Oraimo', 'JBL', 'Vitron'].map((brand) => (
                <label key={brand} className="flex items-center gap-2.5 text-sm text-gray-500 py-2 cursor-pointer">
                  <input
                    type="checkbox"
                    className="rounded border-gray-300"
                    checked={filters.brand.includes(brand)}
                    onChange={(e) => {
                      if (e.target.checked) {
                        setFilters({ ...filters, brand: [...filters.brand, brand] });
                      } else {
                        setFilters({ ...filters, brand: filters.brand.filter((b) => b !== brand) });
                      }
                    }}
                  />
                  {brand}
                </label>
              ))}
            </div>

            <div className="mb-4">
              <p className="text-xs text-gray-500 mb-1.5">Price</p>
              {['Under KSh 5,000', 'KSh 5,000-20,000', 'KSh 20,000+'].map((range) => (
                <label key={range} className="flex items-center gap-2.5 text-sm text-gray-500 py-2 cursor-pointer">
                  <input
                    type="checkbox"
                    className="rounded border-gray-300"
                    checked={filters.priceRange === range}
                    onChange={() => setFilters({ ...filters, priceRange: filters.priceRange === range ? '' : range })}
                  />
                  {range}
                </label>
              ))}
            </div>

            <div>
              <p className="text-xs text-gray-500 mb-1.5">Availability</p>
              <label className="flex items-center gap-2.5 text-sm text-gray-500 py-2 cursor-pointer">
                <input
                  type="checkbox"
                  className="rounded border-gray-300"
                  checked={filters.inStock}
                  onChange={(e) => setFilters({ ...filters, inStock: e.target.checked })}
                />
                In stock
              </label>
            </div>
          </aside>

          {/* Products Grid */}
          <section>
            {loading ? (
              <div className="text-center py-12 text-gray-500">Loading...</div>
            ) : products.length === 0 ? (
              <div className="text-center py-12 text-gray-500">No products found</div>
            ) : (
              <div className="grid grid-cols-1 sm:grid-cols-2 lg:grid-cols-3 gap-6">
                {products.map((product) => (
                  <ProductCard key={product.id} product={product} />
                ))}
              </div>
            )}
          </section>
        </div>
      </div>
    </Layout>
  );
}
