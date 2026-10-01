import { useState, useEffect } from 'react';
import { useParams, Link } from 'react-router-dom';
import Layout from '../components/Layout';
import ProductCard from '../components/ProductCard';
import SEO from '../components/SEO';
import { api } from '../lib/api';

const CATEGORY_NAMES = {
  'mobile-phones': 'Mobile Phones',
  'audio-devices': 'Audio Devices',
  'television-products': 'Television Products',
  'charging-accessories': 'Charging Accessories',
  'computers-accessories': 'Computers & Accessories',
  'electrical-products': 'Electrical Products',
  'security-products': 'Security Products',
  'networking-products': 'Networking Products',
  'automotive-products': 'Automotive Products',
  'gaming-products': 'Gaming Products',
  watches: 'Watches',
  'renewable-energy': 'Renewable Energy',
  'home-appliances': 'Home Appliances',
  'other-electrical': 'Other Electrical Products',
  phones: 'Mobile Phones',
  'laptops-computers': 'Computers & Accessories',
  'tvs-displays': 'Television Products',
  audio: 'Audio Devices',
  solar: 'Renewable Energy',
  'electrical-installation': 'Electrical Products',
  'gaming-devices': 'Gaming Products',
  accessories: 'Charging Accessories',
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
  const [currentPage, setCurrentPage] = useState(1);
  const itemsPerPage = 12;

  useEffect(() => {
    async function fetchCategoryData() {
      setLoading(true);
      setCurrentPage(1);
      try {
        const categories = await api.getCategories();
        const foundCategory = categories.find((c) => c.slug === slug);
        if (foundCategory) setCategory(foundCategory);
        const productsData = await api.getProducts(slug);
        if (productsData) {
          const list = Array.isArray(productsData) ? productsData : (productsData.products || []);
          setProducts(list);
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

  const filteredProducts = products.filter((product) => {
    if (filters.brand.length > 0 && !filters.brand.includes(product.brand)) return false;
    if (filters.inStock && !product.in_stock && product.stock_status !== 'In Stock') return false;
    if (filters.priceRange) {
      if (filters.priceRange === 'Under KSh 5,000' && product.price >= 5000) return false;
      if (filters.priceRange === 'KSh 5,000-20,000' && (product.price < 5000 || product.price > 20000)) return false;
      if (filters.priceRange === 'KSh 20,000+' && product.price <= 20000) return false;
    }
    return true;
  });

  const availableBrands = [...new Set(products.map((p) => p.brand).filter(Boolean))];
  const totalPages = Math.ceil(filteredProducts.length / itemsPerPage);

  return (
    <Layout>
      <SEO
        title={categoryTitle}
        description={`Browse our premium selection of ${categoryTitle} at Naoja Ventures in Lurambi, Kakamega. Best prices, warranty, fast delivery & M-Pesa payments.`}
      />
      <div className="bg-surface min-h-screen">
        <div className="max-w-7xl mx-auto px-4 py-5">
          {/* Breadcrumb */}
          <div className="text-xs text-gray-500 pb-4 flex items-center gap-1.5">
            <Link to="/" className="hover:text-brand-500 transition-colors">Home</Link>
            <span className="text-gray-300">›</span>
            <span className="text-gray-900 font-semibold">{categoryTitle}</span>
          </div>

          {/* Category Header */}
          <div className="flex flex-wrap justify-between items-end gap-4 mb-6">
            <div>
              <h1 className="text-2xl lg:text-3xl font-bold text-gray-900">{categoryTitle}</h1>
              <p className="text-xs text-gray-500 mt-1">
                {filteredProducts.length} products found · Free delivery ≥ KSh 600 · Pickup in 1-2 hours
              </p>
            </div>
            <div className="bg-white border border-gray-200 rounded-lg px-3.5 py-2 text-xs shadow-sm">
              <span className="text-gray-500">Sort by:</span>
              <strong className="ml-1.5 text-gray-900">Recommended</strong>
            </div>
          </div>

          <div className="grid lg:grid-cols-[240px_1fr] gap-6 pb-8">
            {/* Filters Sidebar */}
            <aside className="hidden lg:block bg-white border border-gray-200 rounded-xl p-5 self-start shadow-sm">
              <div className="flex items-center justify-between mb-4 pb-3 border-b border-gray-100">
                <h4 className="font-bold text-gray-900 text-sm">Filters</h4>
                <button
                  onClick={() => setFilters({ brand: [], priceRange: '', inStock: false })}
                  className="text-[11px] text-brand-500 hover:underline cursor-pointer font-semibold"
                >
                  Clear All
                </button>
              </div>

              {availableBrands.length > 0 && (
                <div className="mb-5">
                  <p className="text-xs font-bold uppercase text-gray-400 mb-2 tracking-wide">Brand</p>
                  <div className="space-y-1.5 max-h-48 overflow-y-auto pr-1">
                    {availableBrands.map((brand) => (
                      <label key={brand} className="flex items-center gap-2.5 text-sm text-gray-600 cursor-pointer py-0.5 hover:text-brand-500 transition-colors">
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

              <div className="mb-5">
                <p className="text-xs font-bold uppercase text-gray-400 mb-2 tracking-wide">Price</p>
                <div className="space-y-1.5">
                  {['Under KSh 5,000', 'KSh 5,000-20,000', 'KSh 20,000+'].map((range) => (
                    <label key={range} className="flex items-center gap-2.5 text-sm text-gray-600 cursor-pointer py-0.5 hover:text-brand-500 transition-colors">
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

              <div>
                <p className="text-xs font-bold uppercase text-gray-400 mb-2 tracking-wide">Availability</p>
                <label className="flex items-center gap-2.5 text-sm text-gray-600 cursor-pointer hover:text-brand-500 transition-colors">
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

            {/* Products Grid */}
            <section>
              {loading ? (
                <div className="grid grid-cols-2 sm:grid-cols-3 lg:grid-cols-4 gap-4">
                  {[...Array(8)].map((_, i) => (
                    <div key={i} className="bg-white rounded-xl border border-gray-200 animate-pulse">
                      <div className="aspect-square bg-gray-100 rounded-t-xl" />
                      <div className="p-3 space-y-2">
                        <div className="h-3 bg-gray-100 rounded w-3/4" />
                        <div className="h-4 bg-gray-100 rounded w-1/2" />
                      </div>
                    </div>
                  ))}
                </div>
              ) : filteredProducts.length === 0 ? (
                <div className="text-center py-16 text-gray-500 bg-white border border-dashed border-gray-200 rounded-xl">
                  <p className="font-semibold text-gray-800">No products found</p>
                  <button
                    onClick={() => setFilters({ brand: [], priceRange: '', inStock: false })}
                    className="mt-3 text-sm text-brand-500 font-medium hover:underline cursor-pointer"
                  >
                    Clear Filters
                  </button>
                </div>
              ) : (
                <>
                  <div className="grid grid-cols-2 sm:grid-cols-3 lg:grid-cols-4 gap-4">
                    {filteredProducts
                      .slice((currentPage - 1) * itemsPerPage, currentPage * itemsPerPage)
                      .map((product) => (
                        <ProductCard key={product.id} product={product} />
                      ))}
                  </div>

                  {totalPages > 1 && (
                    <div className="flex items-center justify-center gap-2 mt-8 pt-6 border-t border-gray-100">
                      <button
                        onClick={() => { setCurrentPage((p) => Math.max(1, p - 1)); window.scrollTo({ top: 0, behavior: 'smooth' }); }}
                        disabled={currentPage === 1}
                        className="px-3.5 py-1.5 bg-white border border-gray-200 rounded-lg text-xs font-semibold text-gray-700 disabled:opacity-40 hover:bg-gray-50 transition-colors shadow-sm"
                      >
                        Previous
                      </button>
                      {Array.from({ length: totalPages }, (_, i) => i + 1).map((p) => (
                        <button
                          key={p}
                          onClick={() => { setCurrentPage(p); window.scrollTo({ top: 0, behavior: 'smooth' }); }}
                          className={`w-8 h-8 rounded-lg text-xs font-bold transition-all shadow-sm ${currentPage === p
                              ? 'bg-brand-500 text-white'
                              : 'bg-white text-gray-600 border border-gray-200 hover:bg-gray-50'
                            }`}
                        >
                          {p}
                        </button>
                      ))}
                      <button
                        onClick={() => { setCurrentPage((p) => Math.min(totalPages, p + 1)); window.scrollTo({ top: 0, behavior: 'smooth' }); }}
                        disabled={currentPage === totalPages}
                        className="px-3.5 py-1.5 bg-white border border-gray-200 rounded-lg text-xs font-semibold text-gray-700 disabled:opacity-40 hover:bg-gray-50 transition-colors shadow-sm"
                      >
                        Next
                      </button>
                    </div>
                  )}
                </>
              )}
            </section>
          </div>
        </div>
      </div>
    </Layout>
  );
}
