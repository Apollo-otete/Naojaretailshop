import { Link, useLocation, useNavigate } from 'react-router-dom';
import { 
  ShoppingCart, 
  User, 
  Search, 
  Menu, 
  X, 
  ChevronDown, 
  Shield, 
  Phone, 
  MapPin, 
  Zap, 
  Flame, 
  Headphones,
  CheckCircle2,
  Sparkles
} from 'lucide-react';
import { useState, useEffect, useRef } from 'react';
import { useCart } from '../lib/cart';
import { api } from '../lib/api';

const DEFAULT_CATEGORIES = [
  { id: 1, name: 'Mobile Phones', slug: 'mobile-phones', icon: '📱' },
  { id: 2, name: 'Audio Devices', slug: 'audio-devices', icon: '🎧' },
  { id: 3, name: 'Television Products', slug: 'television-products', icon: '📺' },
  { id: 4, name: 'Charging Accessories', slug: 'charging-accessories', icon: '🔌' },
  { id: 5, name: 'Computers & Accessories', slug: 'computers-accessories', icon: '💻' },
  { id: 6, name: 'Electrical Products', slug: 'electrical-products', icon: '⚡' },
  { id: 7, name: 'Security Products', slug: 'security-products', icon: '🔒' },
  { id: 8, name: 'Renewable Energy', slug: 'renewable-energy', icon: '☀️' },
];

export default function Header() {
  const navigate = useNavigate();
  const { itemCount, total } = useCart();
  const [mobileMenuOpen, setMobileMenuOpen] = useState(false);
  const [categoriesMenuOpen, setCategoriesMenuOpen] = useState(false);
  const [searchQuery, setSearchQuery] = useState('');
  const [categories, setCategories] = useState(DEFAULT_CATEGORIES);
  const dropdownRef = useRef(null);

  useEffect(() => {
    async function fetchCategories() {
      try {
        const data = await api.getCategories();
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
    function handleClickOutside(event) {
      if (dropdownRef.current && !dropdownRef.current.contains(event.target)) {
        setCategoriesMenuOpen(false);
      }
    }
    document.addEventListener('mousedown', handleClickOutside);
    return () => document.removeEventListener('mousedown', handleClickOutside);
  }, []);

  const handleSearchSubmit = (e) => {
    if (e.key === 'Enter' || e.type === 'click') {
      if (searchQuery.trim()) {
        navigate(`/?search=${encodeURIComponent(searchQuery.trim())}`);
        setMobileMenuOpen(false);
      } else {
        navigate('/');
      }
    }
  };

  const whatsappNumber = import.meta.env.VITE_WHATSAPP_NUMBER || '254712345678';

  return (
    <>
      {/* ── 1. TOP ANNOUNCEMENT & TRUST STRIP (Jumia / Kilimall Style) ── */}
      <div className="bg-[#1e2329] text-gray-200 text-xs py-1.5 border-b border-gray-800">
        <div className="max-w-7xl mx-auto px-4 flex flex-wrap items-center justify-between gap-2">
          <div className="flex items-center gap-4 text-[11px] font-medium tracking-wide">
            <span className="flex items-center gap-1 text-[#ff8a00]">
              <Sparkles className="w-3.5 h-3.5" />
              <span>100% Genuine Electronics</span>
            </span>
            <span className="hidden sm:inline text-gray-500">•</span>
            <span className="hidden sm:flex items-center gap-1 text-gray-300">
              <Zap className="w-3.5 h-3.5 text-emerald-400" />
              <span>Same-Day Delivery in Kakamega & Lurambi</span>
            </span>
            <span className="hidden md:inline text-gray-500">•</span>
            <span className="hidden md:flex items-center gap-1 text-emerald-400 font-semibold">
              <span>Lipa Na M-Pesa (Till: 4149288)</span>
            </span>
          </div>

          <div className="flex items-center gap-4 text-[11px]">
            <span className="hidden lg:flex items-center gap-1 text-gray-400">
              <MapPin className="w-3 h-3 text-[#ff8a00]" />
              <span>Lurambi, Opp. Bamboo</span>
            </span>
            <a 
              href={`https://wa.me/${whatsappNumber}`} 
              target="_blank" 
              rel="noopener noreferrer"
              className="flex items-center gap-1 text-emerald-400 hover:text-emerald-300 font-semibold transition-colors"
            >
              <Phone className="w-3 h-3" />
              <span>+254 712 345 678</span>
            </a>
            <Link to="/account?tab=support" className="text-gray-400 hover:text-white transition-colors">
              Help Center
            </Link>
          </div>
        </div>
      </div>

      {/* ── 2. MAIN MARKETPLACE HEADER (Kilimall / AliExpress / Jumia Style) ── */}
      <header className="sticky top-0 z-40 bg-white border-b border-gray-200 shadow-sm">
        <div className="max-w-7xl mx-auto px-4">
          <div className="h-18 lg:h-20 flex items-center justify-between gap-3 lg:gap-8">
            
            {/* Logo */}
            <div className="flex items-center gap-2">
              <Link to="/" className="flex items-center gap-1.5 group">
                <div className="w-10 h-10 rounded-xl bg-gradient-to-tr from-brand-600 to-[#ff6b00] flex items-center justify-center text-white font-black text-xl shadow-md group-hover:scale-105 transition-transform">
                  N
                </div>
                <div className="flex flex-col">
                  <span className="text-2xl font-black tracking-tight text-gray-950 flex items-center gap-1">
                    NAOJA
                    <span className="text-xs uppercase bg-brand-500 text-white font-extrabold px-1.5 py-0.5 rounded tracking-normal">
                      Retail
                    </span>
                  </span>
                  <span className="text-[10px] uppercase font-bold tracking-widest text-gray-600 -mt-1">
                    Ventures Kakamega
                  </span>
                </div>
              </Link>
            </div>

            {/* Prominent Search Bar (Kilimall / Jumia Style) */}
            <div className="flex-1 max-w-2xl hidden md:flex items-center">
              <div className="w-full flex items-center border-2 border-brand-500 rounded-xl overflow-hidden shadow-sm hover:border-brand-600 transition-colors bg-white">
                <div className="pl-4 pr-2 text-gray-400">
                  <Search className="w-5 h-5" />
                </div>
                <input
                  type="text"
                  placeholder="I'm shopping for smartphones, solar, TVs, audio, chargers..."
                  value={searchQuery}
                  onChange={(e) => setSearchQuery(e.target.value)}
                  onKeyDown={handleSearchSubmit}
                  className="w-full py-2.5 px-2 text-sm text-gray-900 placeholder:text-gray-400 outline-none bg-transparent"
                />
                <button
                  onClick={handleSearchSubmit}
                  className="bg-brand-500 hover:bg-brand-600 text-white px-7 py-3 text-sm font-bold tracking-wide transition-colors flex items-center gap-1.5 cursor-pointer uppercase text-xs"
                >
                  Search
                </button>
              </div>
            </div>

            {/* Right Action Hub */}
            <div className="flex items-center gap-2 sm:gap-4">
              
              {/* Account Dropdown */}
              <Link
                to="/account"
                className="flex items-center gap-2 p-2 hover:bg-gray-50 rounded-xl transition-colors text-gray-700"
              >
                <div className="w-9 h-9 rounded-full bg-gray-100 flex items-center justify-center text-gray-600">
                  <User className="w-5 h-5" />
                </div>
                <div className="hidden lg:flex flex-col text-left">
                  <span className="text-[10px] text-gray-600 leading-tight">Welcome</span>
                  <span className="text-xs font-bold text-gray-900 leading-tight">Account & Orders</span>
                </div>
              </Link>

              {/* Cart Button with Count Badge */}
              <Link
                to="/cart"
                className="flex items-center gap-2.5 p-2 bg-brand-50 hover:bg-brand-100/80 rounded-xl transition-all border border-brand-200/60"
              >
                <div className="relative">
                  <ShoppingCart className="w-6 h-6 text-brand-600" />
                  {itemCount > 0 && (
                    <span className="absolute -top-2 -right-2.5 bg-retail-red text-white text-[11px] font-black w-5 h-5 rounded-full flex items-center justify-center shadow-md animate-pulse">
                      {itemCount}
                    </span>
                  )}
                </div>
                <div className="hidden sm:flex flex-col text-left">
                  <span className="text-[10px] text-brand-700 font-bold uppercase leading-tight">Shopping Cart</span>
                  <span className="text-xs font-black text-brand-900 leading-tight">
                    KSh {total.toLocaleString()}
                  </span>
                </div>
              </Link>

              {/* Mobile Hamburger Menu */}
              <button
                className="md:hidden p-2 rounded-xl text-gray-700 hover:bg-gray-100 transition-colors"
                onClick={() => setMobileMenuOpen(!mobileMenuOpen)}
                aria-label="Toggle navigation menu"
              >
                {mobileMenuOpen ? <X className="w-6 h-6" /> : <Menu className="w-6 h-6" />}
              </button>
            </div>
          </div>

          {/* Mobile Search Bar */}
          <div className="md:hidden pb-3">
            <div className="flex items-center border-2 border-brand-500 rounded-xl overflow-hidden bg-white shadow-sm">
              <input
                type="text"
                placeholder="Search phones, solar, TVs, cables..."
                value={searchQuery}
                onChange={(e) => setSearchQuery(e.target.value)}
                onKeyDown={handleSearchSubmit}
                className="flex-1 py-2 px-3 text-xs text-gray-900 outline-none"
              />
              <button
                onClick={handleSearchSubmit}
                className="bg-brand-500 text-white px-4 py-2.5 text-xs font-bold"
              >
                <Search className="w-4 h-4" />
              </button>
            </div>
          </div>
        </div>

        {/* ── 3. SUB-NAVBAR CATEGORY BAR (AliExpress / Kilimall Style) ── */}
        <div className="bg-gray-50 border-t border-gray-200 hidden md:block">
          <div className="max-w-7xl mx-auto px-4 flex items-center justify-between">
            
            {/* All Categories Dropdown Button */}
            <div className="relative" ref={dropdownRef}>
              <button
                onClick={() => setCategoriesMenuOpen(!categoriesMenuOpen)}
                className="flex items-center gap-2 px-4 py-2.5 bg-brand-500 hover:bg-brand-600 text-white font-bold text-xs uppercase tracking-wider rounded-t-lg transition-colors cursor-pointer shadow-sm"
              >
                <Menu className="w-4 h-4" />
                <span>All Categories</span>
                <ChevronDown className={`w-3.5 h-3.5 transition-transform duration-200 ${categoriesMenuOpen ? 'rotate-180' : ''}`} />
              </button>

              {/* Dropdown Menu */}
              {categoriesMenuOpen && (
                <div className="absolute top-full left-0 w-64 bg-white border border-gray-200 rounded-b-xl shadow-2xl overflow-hidden z-50 divide-y divide-gray-100">
                  <div className="py-2">
                    {categories.map((cat) => (
                      <Link
                        key={cat.id}
                        to={`/category/${cat.slug}`}
                        onClick={() => setCategoriesMenuOpen(false)}
                        className="flex items-center justify-between px-4 py-2.5 hover:bg-brand-50 hover:text-brand-600 text-xs font-semibold text-gray-700 transition-colors"
                      >
                        <span className="flex items-center gap-2.5">
                          <span className="text-base">{cat.icon || '📦'}</span>
                          <span>{cat.name}</span>
                        </span>
                        <span className="text-gray-300 text-xs">›</span>
                      </Link>
                    ))}
                  </div>

                  <div className="p-3 bg-gray-50">
                    <Link
                      to="/admin"
                      onClick={() => setCategoriesMenuOpen(false)}
                      className="flex items-center gap-2 text-xs font-bold text-brand-600 hover:underline"
                    >
                      <Shield className="w-3.5 h-3.5" />
                      Admin Dashboard
                    </Link>
                  </div>
                </div>
              )}
            </div>

            {/* Quick Links / Trending Categories */}
            <div className="flex items-center gap-1 overflow-x-auto text-xs font-semibold text-gray-700 py-1.5">
              <Link 
                to="/?filter=deals" 
                className="flex items-center gap-1 px-3 py-1 text-retail-red font-black hover:bg-red-50 rounded-lg transition-colors uppercase text-[11px]"
              >
                <Flame className="w-4 h-4 fill-retail-red text-retail-red animate-bounce" />
                <span>SuperDeals</span>
              </Link>
              <Link to="/category/mobile-phones" className="px-3 py-1 hover:text-brand-600 hover:bg-white rounded-lg transition-colors">
                Mobile Phones
              </Link>
              <Link to="/category/renewable-energy" className="px-3 py-1 hover:text-brand-600 hover:bg-white rounded-lg transition-colors text-amber-600 font-bold">
                ☀️ Solar & Energy
              </Link>
              <Link to="/category/television-products" className="px-3 py-1 hover:text-brand-600 hover:bg-white rounded-lg transition-colors">
                TVs & Audio
              </Link>
              <Link to="/category/charging-accessories" className="px-3 py-1 hover:text-brand-600 hover:bg-white rounded-lg transition-colors">
                Fast Chargers
              </Link>
              <Link to="/category/electrical-products" className="px-3 py-1 hover:text-brand-600 hover:bg-white rounded-lg transition-colors">
                Electrical & Wiring
              </Link>
              <Link to="/category/computers-accessories" className="px-3 py-1 hover:text-brand-600 hover:bg-white rounded-lg transition-colors">
                Computers
              </Link>
              <Link to="/category/security-products" className="px-3 py-1 hover:text-brand-600 hover:bg-white rounded-lg transition-colors">
                Security CCTV
              </Link>
            </div>

            {/* WhatsApp Direct Help */}
            <a
              href={`https://wa.me/${whatsappNumber}`}
              target="_blank"
              rel="noopener noreferrer"
              className="hidden lg:flex items-center gap-1.5 text-xs font-bold text-emerald-600 hover:text-emerald-700 bg-emerald-50 px-2.5 py-1 rounded-full border border-emerald-200 transition-colors"
            >
              <span className="w-2 h-2 rounded-full bg-emerald-500 animate-ping"></span>
              <span>Online Orders</span>
            </a>
          </div>
        </div>
      </header>

      {/* ── MOBILE SLIDE-OUT MENU ── */}
      {mobileMenuOpen && (
        <div className="fixed inset-0 z-50 bg-black/50 backdrop-blur-sm md:hidden flex">
          <div className="w-4/5 max-w-sm bg-white h-full overflow-y-auto flex flex-col shadow-2xl">
            <div className="p-4 bg-brand-500 text-white flex items-center justify-between">
              <div className="flex items-center gap-2">
                <div className="w-8 h-8 rounded-lg bg-white text-brand-600 font-black flex items-center justify-center">
                  N
                </div>
                <span className="font-bold text-lg">Naoja Ventures</span>
              </div>
              <button onClick={() => setMobileMenuOpen(false)} className="p-1 text-white hover:bg-white/20 rounded-lg">
                <X className="w-6 h-6" />
              </button>
            </div>

            <div className="p-4 space-y-4 flex-1">
              <div className="space-y-1">
                <p className="text-[10px] font-black uppercase tracking-wider text-gray-600 px-2">Store Departments</p>
                {categories.map((cat) => (
                  <Link
                    key={cat.id}
                    to={`/category/${cat.slug}`}
                    onClick={() => setMobileMenuOpen(false)}
                    className="flex items-center gap-3 px-3 py-2 text-sm font-semibold text-gray-800 hover:bg-brand-50 hover:text-brand-600 rounded-lg transition-colors"
                  >
                    <span>{cat.icon || '📦'}</span>
                    <span>{cat.name}</span>
                  </Link>
                ))}
              </div>

              <div className="border-t border-gray-100 pt-3 space-y-1">
                <p className="text-[10px] font-black uppercase tracking-wider text-gray-600 px-2">Customer Account</p>
                <Link
                  to="/account"
                  onClick={() => setMobileMenuOpen(false)}
                  className="block px-3 py-2 text-sm font-semibold text-gray-800 hover:bg-gray-50 rounded-lg"
                >
                  My Orders & Account
                </Link>
                <Link
                  to="/account?tab=support"
                  onClick={() => setMobileMenuOpen(false)}
                  className="block px-3 py-2 text-sm font-semibold text-gray-800 hover:bg-gray-50 rounded-lg"
                >
                  Customer Support
                </Link>
                <Link
                  to="/admin"
                  onClick={() => setMobileMenuOpen(false)}
                  className="flex items-center gap-2 px-3 py-2 text-sm font-bold text-brand-600 hover:bg-brand-50 rounded-lg"
                >
                  <Shield className="w-4 h-4" />
                  Admin Dashboard
                </Link>
              </div>
            </div>

            <div className="p-4 bg-gray-50 border-t border-gray-200 text-xs text-gray-500">
              <p className="font-bold text-gray-900">Naoja Ventures</p>
              <p>Lurambi, Kakamega (Opp. Bamboo)</p>
              <p className="mt-1 font-semibold text-emerald-600">Till: 4149288</p>
            </div>
          </div>
          <div className="flex-1" onClick={() => setMobileMenuOpen(false)}></div>
        </div>
      )}
    </>
  );
}
