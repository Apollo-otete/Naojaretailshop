import { Link, useLocation } from 'react-router-dom';
import { ShoppingCart, User, Heart, Search, Menu, X, ChevronDown, Shield } from 'lucide-react';
import { useState, useEffect, useRef } from 'react';
import { useCart } from '../lib/cart';
import { supabase } from '../lib/supabase';
import { Category } from '../types';

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

export default function Header() {
  const location = useLocation();
  const { itemCount } = useCart();
  const [mobileMenuOpen, setMobileMenuOpen] = useState(false);
  const [categoriesMenuOpen, setCategoriesMenuOpen] = useState(false);
  const [searchQuery, setSearchQuery] = useState('');
  const [categories, setCategories] = useState<Category[]>(DEFAULT_CATEGORIES);
  const dropdownRef = useRef<HTMLDivElement>(null);

  useEffect(() => {
    async function fetchCategories() {
      try {
        const { data } = await supabase.from('categories').select('*').order('name');
        if (data && data.length > 0) {
          setCategories(data);
        }
      } catch (error) {
        console.error('Error fetching categories:', error);
      }
    }
    fetchCategories();
  }, []);

  useEffect(() => {
    function handleClickOutside(event: MouseEvent) {
      if (dropdownRef.current && !dropdownRef.current.contains(event.target as Node)) {
        setCategoriesMenuOpen(false);
      }
    }
    document.addEventListener('mousedown', handleClickOutside);
    return () => document.removeEventListener('mousedown', handleClickOutside);
  }, []);

  return (
    <>
      {/* Announcement Bar */}
      <div className="sticky top-0 z-50 bg-white border-b border-gray-200">
        <div className="max-w-7xl mx-auto px-4">
          <div className="h-9 flex items-center justify-between text-xs text-gray-500">
            <span>Free delivery from <strong className="text-gray-900">KSh 600</strong></span>
            <span className="hidden sm:block">Order before <strong className="text-gray-900">4pm</strong> for <strong className="text-gray-900">same-day delivery</strong> (Sun-Fri)</span>
            <span className="hidden md:block">Pickup: <strong className="text-gray-900">Kakamega (Lurambi, Opp. Bamboo)</strong> - Closed <strong className="text-gray-900">Saturday</strong></span>
          </div>
        </div>
      </div>

      {/* Header */}
      <header className="sticky top-9 z-40 bg-white border-b border-gray-200">
        <div className="max-w-7xl mx-auto px-4">
          <div className="h-[72px] flex items-center gap-4">
            {/* Logo and Categories Dropdown */}
            <div className="flex items-center gap-2">
              <Link to="/" className="font-serif font-semibold text-2xl tracking-tight">
                Naoja <span className="text-brand-500">.</span>
              </Link>

              {/* Categories Dropdown Trigger */}
              <div className="relative" ref={dropdownRef}>
                <button
                  onClick={() => setCategoriesMenuOpen(!categoriesMenuOpen)}
                  className="flex items-center gap-1 px-2 py-1.5 rounded-control hover:bg-surface transition-colors"
                  aria-label="Toggle categories menu"
                >
                  <Menu className="w-5 h-5 text-gray-600" />
                  <span className="text-xs text-gray-600 hidden sm:inline">Categories</span>
                  <ChevronDown className={`w-4 h-4 text-gray-400 transition-transform ${categoriesMenuOpen ? 'rotate-180' : ''}`} />
                </button>

                {/* Categories Dropdown Menu */}
                {categoriesMenuOpen && (
                  <div className="absolute top-full left-0 mt-2 w-72 bg-white border border-gray-200 rounded-card shadow-lg overflow-hidden z-50">
                    <div className="max-h-[400px] overflow-y-auto">
                      {categories.map((cat) => (
                        <Link
                          key={cat.id}
                          to={`/category/${cat.slug}`}
                          onClick={() => setCategoriesMenuOpen(false)}
                          className="flex items-center gap-3 px-4 py-3 hover:bg-surface transition-colors border-b border-gray-100 last:border-b-0"
                        >
                          {cat.image_url ? (
                            <img
                              src={cat.image_url}
                              alt={cat.name}
                              className="w-10 h-10 rounded-lg object-cover"
                            />
                          ) : (
                            <div className="w-10 h-10 bg-surface rounded-lg flex items-center justify-center">
                              <span className="text-xs text-gray-400">No img</span>
                            </div>
                          )}
                          <div className="flex-1 min-w-0">
                            <p className="font-medium text-gray-900 text-sm">{cat.name}</p>
                            {cat.description && (
                              <p className="text-xs text-gray-500 truncate">{cat.description}</p>
                            )}
                          </div>
                        </Link>
                      ))}
                    </div>
                    <div className="border-t border-gray-200">
                      <Link
                        to="/account?tab=support"
                        onClick={() => setCategoriesMenuOpen(false)}
                        className="block px-4 py-3 text-sm text-gray-600 hover:bg-surface transition-colors"
                      >
                        Support
                      </Link>
                      <a
                        href="https://adminnaoja.softr.app"
                        target="_blank"
                        rel="noopener noreferrer"
                        onClick={() => setCategoriesMenuOpen(false)}
                        className="flex items-center gap-2 px-4 py-3 text-sm text-gray-600 hover:bg-surface transition-colors border-t border-gray-100"
                      >
                        <Shield className="w-4 h-4" />
                        Admin Portal
                      </a>
                    </div>
                  </div>
                )}
              </div>
            </div>

            {/* Search */}
            <div className="flex-1 hidden md:flex items-center gap-2.5 border border-gray-200 rounded-control h-11 px-3">
              <Search className="w-4 h-4 text-gray-400" />
              <input
                type="text"
                placeholder="Search phones, TVs, solar, appliances..."
                value={searchQuery}
                onChange={(e) => setSearchQuery(e.target.value)}
                className="flex-1 text-sm outline-none bg-transparent"
              />
            </div>

            {/* Icon Buttons */}
            <div className="flex items-center gap-2">
              <Link
                to="/account"
                className="hidden sm:flex items-center gap-2 px-2.5 py-2.5 rounded-control hover:bg-surface transition-colors"
              >
                <User className="w-5 h-5" />
                <span className="text-sm hidden lg:block">Account</span>
              </Link>

              <Link
                to="/account?tab=wishlist"
                className="hidden sm:flex items-center gap-2 px-2.5 py-2.5 rounded-control hover:bg-surface transition-colors"
              >
                <Heart className="w-5 h-5" />
                <span className="text-sm hidden lg:block">Wishlist</span>
              </Link>

              <Link
                to="/cart"
                className="flex items-center gap-2 px-2.5 py-2.5 rounded-control hover:bg-surface transition-colors"
              >
                <ShoppingCart className="w-5 h-5" />
                {itemCount > 0 && (
                  <span className="bg-brand-500 text-white text-xs font-semibold px-2 py-0.5 rounded-full">
                    {itemCount}
                  </span>
                )}
              </Link>

              <button
                className="md:hidden p-2.5 rounded-control hover:bg-surface transition-colors"
                onClick={() => setMobileMenuOpen(!mobileMenuOpen)}
              >
                {mobileMenuOpen ? <X className="w-5 h-5" /> : <Menu className="w-5 h-5" />}
              </button>
            </div>
          </div>
        </div>
      </header>

      {/* Mobile Menu */}
      {mobileMenuOpen && (
        <div className="fixed inset-0 z-50 bg-white md:hidden">
          <div className="flex items-center justify-between p-4 border-b border-gray-200">
            <Link to="/" className="font-serif font-semibold text-2xl" onClick={() => setMobileMenuOpen(false)}>
              Naoja <span className="text-brand-500">.</span>
            </Link>
            <button onClick={() => setMobileMenuOpen(false)}>
              <X className="w-6 h-6" />
            </button>
          </div>

          <div className="p-4">
            <div className="flex items-center gap-2 border border-gray-200 rounded-control h-11 px-3 mb-4">
              <Search className="w-4 h-4 text-gray-400" />
              <input
                type="text"
                placeholder="Search..."
                className="flex-1 text-sm outline-none bg-transparent"
              />
            </div>

            <div className="space-y-1">
              {categories.map((cat) => (
                <Link
                  key={cat.id}
                  to={`/category/${cat.slug}`}
                  className="block py-2 text-gray-700"
                  onClick={() => setMobileMenuOpen(false)}
                >
                  {cat.name}
                </Link>
              ))}
              <Link
                to="/account"
                className="block py-2 text-gray-700"
                onClick={() => setMobileMenuOpen(false)}
              >
                Account
              </Link>
              <a
                href="https://adminnaoja.softr.app"
                target="_blank"
                rel="noopener noreferrer"
                className="flex items-center gap-2 py-2 text-gray-700"
                onClick={() => setMobileMenuOpen(false)}
              >
                <Shield className="w-4 h-4" />
                Admin Portal
              </a>
            </div>
          </div>
        </div>
      )}
    </>
  );
}
