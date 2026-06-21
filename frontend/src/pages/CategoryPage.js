import { useState, useEffect } from 'react';
import { useParams, Link } from 'react-router-dom';
import Layout from '../components/Layout';
import ProductCard from '../components/ProductCard';
import { api } from '../lib/api';

const CATEGORY_NAMES = {
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
  const [products, setProducts] = useState([]);
  const [category, setCategory] = useState(null);
  const [loading, setLoading] = useState(true);
  const [filters, setFilters] = useState({
    brand: [],
    priceRange: '',
    inStock: false,
  });

  useEffect(() => {
    async function fetchCategoryData() {
      setLoading(true);
      try {
        const categories = await api.getCategories();
        const foundCategory = categories.find((c) => c.slug === slug);
        if (foundCategory) {
          setCategory(foundCategory);
        }

        const productsData = await api.getProducts(slug);
        if (productsData) {
          setProducts(productsData);
        }
      } catch (error) {
        console.error('Error fetching category products:', error);
      } finally {
        setLoading(false);
      }
    }

    fetchCategoryData();
  }, [slug]);

  const categoryTitle = CATEGORY_NAMES[slug] || category?.name || 'Category';

  // Apply filters in memory on the frontend
  const filteredProducts = products.filter((product) => {
    // Brand filter
    if (filters.brand.length > 0 && !filters.brand.includes(product.brand)) {
      return false;
    }

    // Availability filter
    if (filters.inStock && !product.in_stock && product.stock_status !== 'In Stock') {
      return false;
    }

    // Price range filter
    if (filters.priceRange) {
      if (filters.priceRange === 'Under KSh 5,000' && product.price >= 5000) {
        return false;
      }
      if (filters.priceRange === 'KSh 5,000-20,000' && (product.price < 5000 || product.price > 20000)) {
        return false;
      }
      if (filters.priceRange === 'KSh 20,000+' && product.price <= 20000) {
        return false;
      }
    }

    return true;
  });

  // Unique list of brands present in these products for filter checkboxes
  const availableBrands = [...new Set(products.map((p) => p.brand).filter(Boolean))];

  return (
    <Layout>
      <div className="max-w-7xl mx-auto px-4 py-6">
        {/* Breadcrumb */}
        <div className="text-xs text-gray-500 pb-4">
          <Link to="/" className="hover:text-brand-500">Home</Link>
          <span className="mx-2">/</span>
          <span className="text-gray-900">{categoryTitle}</span>
        </div>

        {/* Header */}
        <div className="flex flex-wrap justify-between items-end gap-4 mb-6">
          <div>
            <h1 className="font-serif text-3xl font-bold text-gray-900">{categoryTitle}</h1>
            <p className="text-xs text-gray-500 mt-1">
              {filteredProducts.length} Results - Free delivery &ge; KSh 600 - Pickup ready in 1-2 hours
            </p>
          </div>
          <div className="bg-white border border-gray-200 rounded-control px-3.5 py-2 text-xs">
            <span className="text-gray-500">Sort:</span>
            <strong className="ml-1.5 text-gray-900">Recommended</strong>
          </div>
        </div>

        <div className="grid lg:grid-cols-[260px_1fr] gap-8 pb-8">
          {/* Filters Sidebar */}
          <aside className="hidden lg:block bg-white border border-gray-200 rounded-card p-5 self-start shadow-sm">
            <div className="flex items-center justify-between mb-4 pb-3 border-b border-gray-100">
              <h4 className="font-bold text-gray-900">Filters</h4>
              <button 
                onClick={() => setFilters({ brand: [], priceRange: '', inStock: false })}
                className="text-[11px] text-brand-500 hover:underline cursor-pointer"
              >
                Clear All
              </button>
            </div>

            {/* Brand Filter */}
            {availableBrands.length > 0 && (
              <div className="mb-5">
                <p className="text-xs font-bold uppercase text-gray-400 mb-2">Brand</p>
                <div className="space-y-1.5 max-h-48 overflow-y-auto pr-1">
                  {availableBrands.map((brand) => (
                    <label key={brand} className="flex items-center gap-2.5 text-sm text-gray-600 cursor-pointer py-0.5 hover:text-brand-500">
                      <input
                        type="checkbox"
                        className="rounded border-gray-300 text-brand-600 focus:ring-brand-500"
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
              </div>
            )}

            {/* Price Filter */}
            <div className="mb-5">
              <p className="text-xs font-bold uppercase text-gray-400 mb-2">Price</p>
              <div className="space-y-1.5">
                {['Under KSh 5,000', 'KSh 5,000-20,000', 'KSh 20,000+'].map((range) => (
                  <label key={range} className="flex items-center gap-2.5 text-sm text-gray-600 cursor-pointer py-0.5 hover:text-brand-500">
                    <input
                      type="radio"
                      name="priceRange"
                      className="text-brand-600 focus:ring-brand-500"
                      checked={filters.priceRange === range}
                      onChange={() => setFilters({ ...filters, priceRange: range })}
                    />
                    {range}
                  </label>
                ))}
              </div>
            </div>

            {/* Stock Availability */}
            <div>
              <p className="text-xs font-bold uppercase text-gray-400 mb-2">Availability</p>
              <label className="flex items-center gap-2.5 text-sm text-gray-600 cursor-pointer hover:text-brand-500">
                <input
                  type="checkbox"
                  className="rounded border-gray-300 text-brand-600 focus:ring-brand-500"
                  checked={filters.inStock}
                  onChange={(e) => setFilters({ ...filters, inStock: e.target.checked })}
                />
                In Stock Only
              </label>
            </div>
          </aside>

          {/* Products List */}
          <section>
            {loading ? (
              <div className="text-center py-12 text-gray-500 font-medium">Loading products...</div>
            ) : filteredProducts.length === 0 ? (
              <div className="text-center py-16 text-gray-500 border border-dashed border-gray-200 rounded-card bg-white">
                <p className="font-semibold text-gray-800">No products found matching filters</p>
                <button 
                  onClick={() => setFilters({ brand: [], priceRange: '', inStock: false })}
                  className="mt-3 text-sm text-brand-500 font-medium hover:underline cursor-pointer"
                >
                  Clear Filters
                </button>
              </div>
            ) : (
              <div className="grid grid-cols-1 sm:grid-cols-2 lg:grid-cols-3 gap-6">
                {filteredProducts.map((product) => (
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
