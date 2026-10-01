import { Link, useLocation } from 'react-router-dom';
import { useState, useEffect } from 'react';
import Layout from '../components/Layout';
import ProductCard from '../components/ProductCard';
import { api } from '../lib/api';
import SEO from '../components/SEO';
import { 
  Star, 
  Phone, 
  MessageSquare, 
  ShieldCheck, 
  Truck, 
  CreditCard, 
  ChevronRight, 
  ChevronLeft,
  Flame, 
  Zap, 
  Clock, 
  ArrowRight,
  Sparkles,
  ShoppingBag,
  Award,
  CheckCircle2,
  RefreshCw,
  Send
} from 'lucide-react';

const DEFAULT_CATEGORIES = [
  { id: 1, name: 'Mobile Phones', slug: 'mobile-phones', icon: '📱', color: 'bg-blue-50 text-blue-600' },
  { id: 2, name: 'Solar & Renewable', slug: 'renewable-energy', icon: '☀️', color: 'bg-amber-50 text-amber-600' },
  { id: 3, name: 'TVs & Audio Displays', slug: 'television-products', icon: '📺', color: 'bg-purple-50 text-purple-600' },
  { id: 4, name: 'Fast Chargers & Cables', slug: 'charging-accessories', icon: '🔌', color: 'bg-emerald-50 text-emerald-600' },
  { id: 5, name: 'Computers & Laptops', slug: 'computers-accessories', icon: '💻', color: 'bg-indigo-50 text-indigo-600' },
  { id: 6, name: 'Electrical & Wiring', slug: 'electrical-products', icon: '⚡', color: 'bg-orange-50 text-orange-600' },
  { id: 7, name: 'Security CCTV Systems', slug: 'security-products', icon: '🔒', color: 'bg-red-50 text-red-600' },
  { id: 8, name: 'Audio & Bluetooth', slug: 'audio-devices', icon: '🎧', color: 'bg-pink-50 text-pink-600' },
];

const HERO_SLIDES = [
  {
    id: 1,
    badge: '🔥 PAYDAY SUPER SALE',
    title: 'Smartphones & Solar Tech Deals',
    subtitle: 'Power up your home & life with genuine electronics. Up to 40% OFF this week.',
    tag: 'From KSh 1,299',
    cta: 'Shop Mega Deals',
    link: '#flash-sales',
    bgGradient: 'from-[#e11d48] via-[#ea580c] to-[#f97316]',
    image: 'https://images.unsplash.com/photo-1511707171634-5f897ff02aa9?w=600&auto=format&fit=crop&q=80',
  },
  {
    id: 2,
    badge: '☀️ RENEWABLE ENERGY REVOLUTION',
    title: 'High-Efficiency Solar Inverters & Panels',
    subtitle: 'Zero power blackouts! Reliable solar solutions tailored for Kakamega homes & business premises.',
    tag: 'Save up to 35%',
    cta: 'Explore Solar Range',
    link: '/category/renewable-energy',
    bgGradient: 'from-[#0f766e] via-[#0d9488] to-[#14b8a6]',
    image: 'https://images.unsplash.com/photo-1509391365360-2e959784a276?w=600&auto=format&fit=crop&q=80',
  },
  {
    id: 3,
    badge: '📲 INSTANT M-PESA CHECKOUT',
    title: 'Order Fast, Pay Conveniently',
    subtitle: 'Direct STK Push straight to your phone, or Buy Goods Till 4149288. Pick up ready in 1 hour!',
    tag: 'Till: 4149288',
    cta: 'Browse Catalog',
    link: '#all-products',
    bgGradient: 'from-[#1e3a8a] via-[#2563eb] to-[#3b82f6]',
    image: 'https://images.unsplash.com/photo-1556742049-0a67c5574f73?w=600&auto=format&fit=crop&q=80',
  }
];

export default function HomePage() {
  const [categories, setCategories] = useState(DEFAULT_CATEGORIES);
  const [products, setProducts] = useState([]);
  const [reviews, setReviews] = useState([]);
  const [loading, setLoading] = useState(true);
  const [currentSlide, setCurrentSlide] = useState(0);

  // Countdown timer for Flash Deals (AliExpress / Jumia style)
  const [timeLeft, setTimeLeft] = useState({ hours: 4, minutes: 35, seconds: 22 });

  // Newsletter state
  const [newsName, setNewsName] = useState('');
  const [newsEmail, setNewsEmail] = useState('');
  const [newsStatus, setNewsStatus] = useState({ success: false, message: '' });

  // Review state
  const [revName, setRevName] = useState('');
  const [revRating, setRevRating] = useState(5);
  const [revComment, setRevComment] = useState('');
  const [revStatus, setRevStatus] = useState({ success: false, message: '' });

  const location = useLocation();
  const searchParams = new URLSearchParams(location.search);
  const search = searchParams.get('search') || '';

  // Timer countdown
  useEffect(() => {
    const timer = setInterval(() => {
      setTimeLeft(prev => {
        if (prev.seconds > 0) return { ...prev, seconds: prev.seconds - 1 };
        if (prev.minutes > 0) return { ...prev, minutes: prev.minutes - 1, seconds: 59 };
        if (prev.hours > 0) return { hours: prev.hours - 1, minutes: 59, seconds: 59 };
        return { hours: 6, minutes: 0, seconds: 0 };
      });
    }, 1000);
    return () => clearInterval(timer);
  }, []);

  // Slide rotation
  useEffect(() => {
    const slideTimer = setInterval(() => {
      setCurrentSlide(prev => (prev + 1) % HERO_SLIDES.length);
    }, 6000);
    return () => clearInterval(slideTimer);
  }, []);

  useEffect(() => {
    async function fetchData() {
      setLoading(true);
      try {
        const [categoriesData, productsData, reviewsData] = await Promise.all([
          api.getCategories(),
          api.getProducts('', search),
          api.getReviews(),
        ]);

        if (categoriesData && categoriesData.length > 0) {
          setCategories(categoriesData);
        }
        if (productsData) {
          const list = Array.isArray(productsData) ? productsData : (productsData.products || []);
          setProducts(list);
        }
        if (reviewsData) {
          setReviews(reviewsData);
        }
      } catch (error) {
        console.error('Error fetching home data:', error);
      } finally {
        setLoading(false);
      }
    }

    fetchData();
  }, [search]);

  // Newsletter submit
  const handleNewsletterSubmit = async (e) => {
    e.preventDefault();
    if (!newsName || !newsEmail) return;
    try {
      const res = await api.subscribeNewsletter(newsName, newsEmail);
      setNewsStatus({ success: true, message: res.message || 'Subscribed successfully!' });
      setNewsName('');
      setNewsEmail('');
    } catch (error) {
      setNewsStatus({ success: false, message: error.message || 'Subscription failed.' });
    }
  };

  // Review submit
  const handleReviewSubmit = async (e) => {
    e.preventDefault();
    if (!revName || !revComment) return;
    try {
      const res = await api.submitReview({
        name: revName,
        rating: revRating,
        comment: revComment
      });
      setRevStatus({ 
        success: true, 
        message: res.message || 'Review submitted! It will appear once approved.' 
      });
      setRevName('');
      setRevComment('');
      setRevRating(5);
    } catch (error) {
      setRevStatus({ success: false, message: error.message || 'Failed to submit review.' });
    }
  };

  // Department filters
  const flashSales = products.slice(0, 6);
  const solarProducts = products.filter(p => p.category_id === 8 || p.category_slug === 'renewable-energy' || (p.name && p.name.toLowerCase().includes('solar')));
  const phoneProducts = products.filter(p => p.category_id === 1 || p.category_slug === 'mobile-phones' || (p.name && p.name.toLowerCase().includes('phone')));

  const whatsappNumber = import.meta.env.VITE_WHATSAPP_NUMBER || '254712345678';

  return (
    <Layout>
      <SEO 
        title="Quality Retail & Wholesale Shop"
        description="Shop genuine smartphones, solar equipment, smart TVs, fast chargers, computers, and electrical supplies at Naoja Ventures in Lurambi, Kakamega. Fast delivery & M-Pesa Till 4149288."
      />

      <div className="bg-[#f4f5f8] min-h-screen py-4 lg:py-6">
        <div className="max-w-7xl mx-auto px-3 sm:px-4 space-y-6">

          {/* ══════════════════════════════════════════════════════════════════════
              SECTION 1: THE ICONIC 3-COLUMN MARKETPLACE HERO (Kilimall / Alibaba Style)
             ══════════════════════════════════════════════════════════════════════ */}
          <div className="grid grid-cols-1 lg:grid-cols-12 gap-4">
            
            {/* Left Column: Vertical Category Menu (Kilimall / Alibaba Style) */}
            <aside className="hidden lg:block lg:col-span-3 bg-white rounded-xl border border-gray-200/80 shadow-sm p-2 flex flex-col justify-between">
              <div className="space-y-0.5">
                <div className="px-3 py-2 text-xs font-black uppercase tracking-wider text-gray-500 border-b border-gray-100 flex items-center justify-between">
                  <span>Categories</span>
                  <span className="text-[10px] text-brand-600 bg-brand-50 px-1.5 py-0.5 rounded font-bold">Top Stores</span>
                </div>
                {categories.slice(0, 8).map((cat) => (
                  <Link
                    key={cat.id}
                    to={`/category/${cat.slug}`}
                    className="flex items-center justify-between px-3 py-2.5 rounded-lg text-xs font-bold text-gray-700 hover:bg-brand-50 hover:text-brand-600 transition-colors group"
                  >
                    <div className="flex items-center gap-2.5 truncate">
                      <span className="text-base group-hover:scale-110 transition-transform">{cat.icon || '📦'}</span>
                      <span className="truncate">{cat.name}</span>
                    </div>
                    <ChevronRight className="w-3.5 h-3.5 text-gray-300 group-hover:text-brand-600 transition-colors shrink-0" />
                  </Link>
                ))}
              </div>

              {/* Verified Seller / Till Callout */}
              <div className="mt-2 p-3 bg-gradient-to-br from-emerald-50 to-teal-50 rounded-lg border border-emerald-100 text-xs">
                <div className="flex items-center gap-2 text-emerald-800 font-bold mb-1">
                  <CheckCircle2 className="w-4 h-4 text-emerald-600" />
                  <span>Verified M-Pesa Merchant</span>
                </div>
                <p className="text-[11px] text-emerald-700">
                  Lipa Na M-Pesa Buy Goods Till: <strong className="font-extrabold text-emerald-900">4149288</strong>
                </p>
              </div>
            </aside>

            {/* Center Column: High-Impact Carousel Banner */}
            <div className="lg:col-span-6 relative overflow-hidden rounded-xl shadow-md min-h-[320px] sm:min-h-[380px] lg:min-h-[420px] flex flex-col justify-between">
              {HERO_SLIDES.map((slide, index) => (
                <div
                  key={slide.id}
                  className={`absolute inset-0 bg-gradient-to-r ${slide.bgGradient} text-white p-6 sm:p-8 flex flex-col justify-between transition-opacity duration-700 ${
                    index === currentSlide ? 'opacity-100 z-10 pointer-events-auto' : 'opacity-0 z-0 pointer-events-none'
                  }`}
                >
                  {/* Decorative background image overlay */}
                  <div 
                    className="absolute inset-0 opacity-15 mix-blend-overlay bg-cover bg-center pointer-events-none"
                    style={{ backgroundImage: `url(${slide.image})` }}
                  ></div>

                  <div className="relative z-10 space-y-3 max-w-md">
                    <span className="inline-block bg-white/20 backdrop-blur-sm text-white text-[11px] font-black uppercase tracking-wider px-3 py-1 rounded-full border border-white/30 shadow-sm">
                      {slide.badge}
                    </span>
                    <h2 className="text-2xl sm:text-3xl lg:text-4xl font-black leading-tight text-white drop-shadow-sm">
                      {slide.title}
                    </h2>
                    <p className="text-xs sm:text-sm text-white/90 font-medium leading-relaxed">
                      {slide.subtitle}
                    </p>
                  </div>

                  <div className="relative z-10 pt-4 flex items-center gap-3">
                    {slide.link.startsWith('#') ? (
                      <a
                        href={slide.link}
                        className="bg-white hover:bg-gray-100 text-gray-900 font-black text-xs sm:text-sm px-6 py-3 rounded-xl shadow-lg transition-transform active:scale-95 flex items-center gap-2 uppercase tracking-wide cursor-pointer"
                      >
                        <span>{slide.cta}</span>
                        <ArrowRight className="w-4 h-4" />
                      </a>
                    ) : (
                      <Link
                        to={slide.link}
                        className="bg-white hover:bg-gray-100 text-gray-900 font-black text-xs sm:text-sm px-6 py-3 rounded-xl shadow-lg transition-transform active:scale-95 flex items-center gap-2 uppercase tracking-wide cursor-pointer"
                      >
                        <span>{slide.cta}</span>
                        <ArrowRight className="w-4 h-4" />
                      </Link>
                    )}
                    <span className="text-xs font-bold text-white/90 bg-black/20 px-3 py-2 rounded-lg backdrop-blur-xs">
                      {slide.tag}
                    </span>
                  </div>
                </div>
              ))}

              {/* Carousel Arrows */}
              <button
                onClick={() => setCurrentSlide(prev => (prev - 1 + HERO_SLIDES.length) % HERO_SLIDES.length)}
                className="absolute left-3 top-1/2 -translate-y-1/2 z-20 w-8 h-8 rounded-full bg-black/30 hover:bg-black/50 text-white flex items-center justify-center backdrop-blur-xs transition-colors"
                aria-label="Previous slide"
              >
                <ChevronLeft className="w-5 h-5" />
              </button>
              <button
                onClick={() => setCurrentSlide(prev => (prev + 1) % HERO_SLIDES.length)}
                className="absolute right-3 top-1/2 -translate-y-1/2 z-20 w-8 h-8 rounded-full bg-black/30 hover:bg-black/50 text-white flex items-center justify-center backdrop-blur-xs transition-colors"
                aria-label="Next slide"
              >
                <ChevronRight className="w-5 h-5" />
              </button>

              {/* Dots */}
              <div className="absolute bottom-3 right-4 z-20 flex gap-1.5">
                {HERO_SLIDES.map((_, i) => (
                  <button
                    key={i}
                    onClick={() => setCurrentSlide(i)}
                    className={`h-2 rounded-full transition-all ${
                      i === currentSlide ? 'w-6 bg-white' : 'w-2 bg-white/50'
                    }`}
                    aria-label={`Slide ${i + 1}`}
                  />
                ))}
              </div>
            </div>

            {/* Right Column: Quick Value & Trust Cards (Alibaba / Jumia Style) */}
            <div className="lg:col-span-3 flex flex-col gap-3">
              
              {/* Card 1: Fast M-Pesa STK Push */}
              <div className="bg-white p-4 rounded-xl border border-gray-200/80 shadow-sm flex items-start gap-3.5 hover:shadow-md transition-shadow">
                <div className="w-10 h-10 rounded-xl bg-emerald-50 text-emerald-600 flex items-center justify-center shrink-0 border border-emerald-100">
                  <CreditCard className="w-5 h-5" />
                </div>
                <div>
                  <h3 className="font-extrabold text-xs text-gray-900 uppercase tracking-wide">M-Pesa Express</h3>
                  <p className="text-[11px] text-gray-500 mt-0.5">Prompt appears right on your phone. Safe, zero errors.</p>
                  <span className="text-[10px] text-emerald-600 font-extrabold mt-1 inline-block">Till 4149288</span>
                </div>
              </div>

              {/* Card 2: Lurambi Store Pickup */}
              <div className="bg-white p-4 rounded-xl border border-gray-200/80 shadow-sm flex items-start gap-3.5 hover:shadow-md transition-shadow">
                <div className="w-10 h-10 rounded-xl bg-orange-50 text-brand-600 flex items-center justify-center shrink-0 border border-brand-100">
                  <ShoppingBag className="w-5 h-5" />
                </div>
                <div>
                  <h3 className="font-extrabold text-xs text-gray-900 uppercase tracking-wide">Store Pickup</h3>
                  <p className="text-[11px] text-gray-500 mt-0.5">Ready in 1-2 hours. Opp. Bamboo, Lurambi, Kakamega.</p>
                  <span className="text-[10px] text-brand-600 font-extrabold mt-1 inline-block">Free Pickup</span>
                </div>
              </div>

              {/* Card 3: Fast Delivery in Kakamega */}
              <div className="bg-white p-4 rounded-xl border border-gray-200/80 shadow-sm flex items-start gap-3.5 hover:shadow-md transition-shadow">
                <div className="w-10 h-10 rounded-xl bg-blue-50 text-blue-600 flex items-center justify-center shrink-0 border border-blue-100">
                  <Truck className="w-5 h-5" />
                </div>
                <div>
                  <h3 className="font-extrabold text-xs text-gray-900 uppercase tracking-wide">Same-Day Delivery</h3>
                  <p className="text-[11px] text-gray-500 mt-0.5">Direct dispatch in Kakamega town, MMUST & environs.</p>
                  <span className="text-[10px] text-blue-600 font-extrabold mt-1 inline-block">Free &ge; KSh 600</span>
                </div>
              </div>

              {/* Card 4: Verified Warranty */}
              <div className="bg-white p-4 rounded-xl border border-gray-200/80 shadow-sm flex items-start gap-3.5 hover:shadow-md transition-shadow">
                <div className="w-10 h-10 rounded-xl bg-purple-50 text-purple-600 flex items-center justify-center shrink-0 border border-purple-100">
                  <ShieldCheck className="w-5 h-5" />
                </div>
                <div>
                  <h3 className="font-extrabold text-xs text-gray-900 uppercase tracking-wide">Genuine Warranty</h3>
                  <p className="text-[11px] text-gray-500 mt-0.5">Every device tested before packing with replacement guarantee.</p>
                </div>
              </div>
            </div>

          </div>

          {/* ══════════════════════════════════════════════════════════════════════
              SECTION 2: CIRCULAR DEPARTMENT BADGES ROW (Kilimall Style)
             ══════════════════════════════════════════════════════════════════════ */}
          <div className="bg-white p-4 rounded-xl border border-gray-200/80 shadow-sm">
            <div className="grid grid-cols-4 sm:grid-cols-8 gap-3 sm:gap-4 text-center">
              {DEFAULT_CATEGORIES.map((item) => (
                <Link
                  key={item.id}
                  to={`/category/${item.slug}`}
                  className="flex flex-col items-center group cursor-pointer"
                >
                  <div className={`w-13 h-13 sm:w-16 sm:h-16 rounded-full flex items-center justify-center text-2xl sm:text-3xl ${item.color} shadow-sm group-hover:scale-110 group-hover:shadow-md transition-all duration-200 mb-1.5`}>
                    {item.icon}
                  </div>
                  <span className="text-[11px] sm:text-xs font-bold text-gray-800 group-hover:text-brand-600 transition-colors line-clamp-1">
                    {item.name}
                  </span>
                </Link>
              ))}
            </div>
          </div>

          {/* ══════════════════════════════════════════════════════════════════════
              SECTION 3: SUPERDEALS & FLASH SALES STRIP (AliExpress / Jumia Style)
             ══════════════════════════════════════════════════════════════════════ */}
          <section id="flash-sales" className="bg-white rounded-xl border border-gray-200/80 shadow-sm overflow-hidden">
            {/* Header with live timer */}
            <div className="bg-gradient-to-r from-retail-red via-[#ea580c] to-[#ff6b00] px-4 py-3 text-white flex flex-wrap items-center justify-between gap-3">
              <div className="flex items-center gap-3">
                <div className="flex items-center gap-1.5 bg-black/20 px-2.5 py-1 rounded-md">
                  <Flame className="w-5 h-5 fill-yellow-300 text-yellow-300 animate-bounce" />
                  <span className="font-black text-sm sm:text-base tracking-wide uppercase">FLASH SALES</span>
                </div>
                <div className="hidden sm:flex items-center gap-1.5 text-xs font-bold text-white/95">
                  <Clock className="w-4 h-4" />
                  <span>Time Left:</span>
                  <div className="flex items-center gap-1 font-mono font-black text-xs">
                    <span className="bg-black/30 px-1.5 py-0.5 rounded">{String(timeLeft.hours).padStart(2, '0')}h</span>:
                    <span className="bg-black/30 px-1.5 py-0.5 rounded">{String(timeLeft.minutes).padStart(2, '0')}m</span>:
                    <span className="bg-black/30 px-1.5 py-0.5 rounded">{String(timeLeft.seconds).padStart(2, '0')}s</span>
                  </div>
                </div>
              </div>

              <Link
                to="/category/mobile-phones"
                className="text-xs font-extrabold text-white hover:text-yellow-200 flex items-center gap-1 uppercase tracking-wider"
              >
                <span>View More Deals</span>
                <ChevronRight className="w-4 h-4" />
              </Link>
            </div>

            {/* Products Grid */}
            <div className="p-4">
              {loading ? (
                <div className="py-12 text-center text-xs font-semibold text-gray-400">Loading live deals...</div>
              ) : flashSales.length === 0 ? (
                <div className="py-8 text-center text-xs text-gray-400">No active flash deals at the moment.</div>
              ) : (
                <div className="grid grid-cols-2 sm:grid-cols-3 md:grid-cols-4 lg:grid-cols-6 gap-3">
                  {flashSales.map((product) => (
                    <ProductCard key={product.id} product={product} />
                  ))}
                </div>
              )}
            </div>
          </section>

          {/* ══════════════════════════════════════════════════════════════════════
              SECTION 4: SOLAR & RENEWABLE ENERGY SHOWCASE (Key Kakamega Category)
             ══════════════════════════════════════════════════════════════════════ */}
          {solarProducts.length > 0 && (
            <section className="bg-white rounded-xl border border-gray-200/80 shadow-sm p-4 sm:p-6 space-y-4">
              <div className="flex items-center justify-between border-b border-gray-100 pb-3">
                <div className="flex items-center gap-2">
                  <div className="w-8 h-8 rounded-lg bg-amber-500 text-white flex items-center justify-center font-bold">
                    ☀️
                  </div>
                  <div>
                    <h3 className="font-black text-base sm:text-lg text-gray-900 tracking-tight">Solar Panels, Batteries & Inverters</h3>
                    <p className="text-xs text-gray-500">Uninterrupted clean power solutions for Western Kenya</p>
                  </div>
                </div>
                <Link
                  to="/category/renewable-energy"
                  className="text-xs font-bold text-brand-600 hover:text-brand-700 flex items-center gap-0.5"
                >
                  <span>See All</span>
                  <ChevronRight className="w-4 h-4" />
                </Link>
              </div>

              <div className="grid grid-cols-2 sm:grid-cols-3 md:grid-cols-4 lg:grid-cols-6 gap-3">
                {solarProducts.slice(0, 6).map((product) => (
                  <ProductCard key={product.id} product={product} />
                ))}
              </div>
            </section>
          )}

          {/* ══════════════════════════════════════════════════════════════════════
              SECTION 5: ALL PRODUCTS & GENERAL MERCHANDISE
             ══════════════════════════════════════════════════════════════════════ */}
          <section id="all-products" className="bg-white rounded-xl border border-gray-200/80 shadow-sm p-4 sm:p-6 space-y-4">
            <div className="flex items-center justify-between border-b border-gray-100 pb-3">
              <div className="flex items-center gap-2">
                <div className="w-8 h-8 rounded-lg bg-brand-500 text-white flex items-center justify-center font-bold">
                  ⚡
                </div>
                <div>
                  <h3 className="font-black text-base sm:text-lg text-gray-900 tracking-tight">Trending in Kakamega</h3>
                  <p className="text-xs text-gray-500">Fastest-moving smartphones, electrical cables and household accessories</p>
                </div>
              </div>
              <span className="text-xs font-bold text-gray-400">
                {products.length} Items Available
              </span>
            </div>

            {loading ? (
              <div className="py-12 text-center text-xs font-semibold text-gray-400">Loading products...</div>
            ) : products.length === 0 ? (
              <div className="py-12 text-center text-sm text-gray-500">No products found matching your search.</div>
            ) : (
              <div className="grid grid-cols-2 sm:grid-cols-3 md:grid-cols-4 lg:grid-cols-4 gap-4">
                {products.map((product) => (
                  <ProductCard key={product.id} product={product} />
                ))}
              </div>
            )}
          </section>

          {/* ══════════════════════════════════════════════════════════════════════
              SECTION 6: CUSTOMER REVIEWS & TESTIMONIALS
             ══════════════════════════════════════════════════════════════════════ */}
          <section className="bg-white rounded-xl border border-gray-200/80 shadow-sm p-4 sm:p-6 space-y-6">
            <div className="flex items-center justify-between border-b border-gray-100 pb-3">
              <div>
                <h3 className="font-black text-base sm:text-lg text-gray-900 tracking-tight">Verified Buyer Reviews</h3>
                <p className="text-xs text-gray-500">What Kakamega shoppers say about Naoja Ventures</p>
              </div>
              <div className="flex items-center gap-1 text-amber-500 font-extrabold text-sm">
                <Star className="w-4 h-4 fill-amber-400" />
                <span>4.9 / 5.0 Store Rating</span>
              </div>
            </div>

            <div className="grid grid-cols-1 md:grid-cols-3 gap-4">
              {reviews.length > 0 ? (
                reviews.slice(0, 3).map((rev) => (
                  <div key={rev.id} className="p-4 rounded-xl bg-gray-50 border border-gray-100 space-y-2.5 flex flex-col justify-between">
                    <div>
                      <div className="flex items-center justify-between mb-1">
                        <div className="flex text-amber-400">
                          {[...Array(5)].map((_, i) => (
                            <Star 
                              key={i} 
                              className={`w-3.5 h-3.5 fill-current ${i < rev.rating ? 'text-amber-400' : 'text-gray-200'}`} 
                            />
                          ))}
                        </div>
                        <span className="text-[10px] text-emerald-700 bg-emerald-50 px-2 py-0.5 rounded font-bold border border-emerald-100">
                          Verified Buyer
                        </span>
                      </div>
                      <p className="text-xs text-gray-700 font-medium leading-relaxed italic">
                        "{rev.comment}"
                      </p>
                    </div>
                    <div className="pt-2 border-t border-gray-200/50 flex items-center justify-between text-[11px] text-gray-500">
                      <strong className="text-gray-900">{rev.name}</strong>
                      <span>{new Date(rev.created_at || Date.now()).toLocaleDateString()}</span>
                    </div>
                  </div>
                ))
              ) : (
                <>
                  <div className="p-4 rounded-xl bg-gray-50 border border-gray-100 space-y-2">
                    <div className="flex text-amber-400">
                      {[...Array(5)].map((_, i) => (
                        <Star key={i} className="w-3.5 h-3.5 fill-amber-400 text-amber-400" />
                      ))}
                    </div>
                    <p className="text-xs text-gray-700 font-medium italic">
                      "I bought a solar charge controller and battery setup here in Lurambi. Fast delivery and original warranty. Highly recommended!"
                    </p>
                    <div className="text-[11px] text-gray-500 pt-2 border-t border-gray-200/50">
                      <strong className="text-gray-900">Brian O.</strong> — Kakamega
                    </div>
                  </div>
                  <div className="p-4 rounded-xl bg-gray-50 border border-gray-100 space-y-2">
                    <div className="flex text-amber-400">
                      {[...Array(5)].map((_, i) => (
                        <Star key={i} className="w-3.5 h-3.5 fill-amber-400 text-amber-400" />
                      ))}
                    </div>
                    <p className="text-xs text-gray-700 font-medium italic">
                      "Ordered a fast charger and extension cable online. Prompt appeared on my phone for M-Pesa immediately and item was ready in 30 mins."
                    </p>
                    <div className="text-[11px] text-gray-500 pt-2 border-t border-gray-200/50">
                      <strong className="text-gray-900">Mercy W.</strong> — MMUST Area
                    </div>
                  </div>
                  <div className="p-4 rounded-xl bg-gray-50 border border-gray-100 space-y-2">
                    <div className="flex text-amber-400">
                      {[...Array(5)].map((_, i) => (
                        <Star key={i} className="w-3.5 h-3.5 fill-amber-400 text-amber-400" />
                      ))}
                    </div>
                    <p className="text-xs text-gray-700 font-medium italic">
                      "Quality electrical switches and bulb sockets. The prices are way better than town centre shops. Will buy again."
                    </p>
                    <div className="text-[11px] text-gray-500 pt-2 border-t border-gray-200/50">
                      <strong className="text-gray-900">Dan K.</strong> — Lurambi
                    </div>
                  </div>
                </>
              )}
            </div>

            {/* Leave a review form */}
            <div className="pt-4 border-t border-gray-100">
              <form onSubmit={handleReviewSubmit} className="bg-gray-50/70 p-4 rounded-xl border border-gray-200/70 space-y-3">
                <div className="flex items-center justify-between">
                  <h4 className="text-xs font-extrabold uppercase tracking-wider text-gray-700">Leave a Review for Naoja Ventures</h4>
                  <div className="flex items-center gap-1">
                    {[1, 2, 3, 4, 5].map((star) => (
                      <button
                        type="button"
                        key={star}
                        onClick={() => setRevRating(star)}
                        className="cursor-pointer"
                      >
                        <Star 
                          className={`w-4 h-4 ${star <= revRating ? 'fill-amber-400 text-amber-400' : 'text-gray-300'}`} 
                        />
                      </button>
                    ))}
                  </div>
                </div>

                {revStatus.message && (
                  <p className={`text-xs font-semibold ${revStatus.success ? 'text-emerald-600' : 'text-red-600'}`}>
                    {revStatus.message}
                  </p>
                )}

                <div className="grid grid-cols-1 sm:grid-cols-3 gap-3">
                  <input
                    type="text"
                    required
                    placeholder="Your Name"
                    value={revName}
                    onChange={(e) => setRevName(e.target.value)}
                    className="h-10 px-3 border border-gray-200 rounded-lg text-xs bg-white outline-none focus:border-brand-500"
                  />
                  <input
                    type="text"
                    required
                    placeholder="Your Feedback / Comment"
                    value={revComment}
                    onChange={(e) => setRevComment(e.target.value)}
                    className="h-10 px-3 border border-gray-200 rounded-lg text-xs bg-white outline-none focus:border-brand-500 sm:col-span-2"
                  />
                </div>

                <div className="flex justify-end">
                  <button
                    type="submit"
                    className="h-9 px-4 bg-brand-500 hover:bg-brand-600 text-white font-bold text-xs rounded-lg transition-colors cursor-pointer"
                  >
                    Submit Review
                  </button>
                </div>
              </form>
            </div>
          </section>

          {/* ══════════════════════════════════════════════════════════════════════
              SECTION 7: NEWSLETTER & DEALS ALERT BAR
             ══════════════════════════════════════════════════════════════════════ */}
          <div className="bg-gradient-to-r from-gray-900 to-[#1e293b] rounded-xl p-6 text-white flex flex-col md:flex-row items-center justify-between gap-6 shadow-md">
            <div className="space-y-1 text-center md:text-left">
              <span className="text-[10px] font-black uppercase tracking-widest text-[#ff8a00]">Exclusive Kakamega Offers</span>
              <h3 className="text-xl sm:text-2xl font-black">Subscribe for New Stock & Flash Deals</h3>
              <p className="text-xs text-gray-400">Get notified when solar supplies, smart TVs and smartphones arrive.</p>
            </div>

            <form onSubmit={handleNewsletterSubmit} className="w-full md:w-auto flex flex-col sm:flex-row gap-2 max-w-md">
              <input
                type="text"
                required
                placeholder="Your Name"
                value={newsName}
                onChange={(e) => setNewsName(e.target.value)}
                className="h-11 px-3.5 bg-gray-800 border border-gray-700 text-white placeholder:text-gray-500 rounded-lg text-xs outline-none focus:border-brand-500"
              />
              <input
                type="email"
                required
                placeholder="Email Address"
                value={newsEmail}
                onChange={(e) => setNewsEmail(e.target.value)}
                className="h-11 px-3.5 bg-gray-800 border border-gray-700 text-white placeholder:text-gray-500 rounded-lg text-xs outline-none focus:border-brand-500"
              />
              <button
                type="submit"
                className="h-11 px-5 bg-brand-500 hover:bg-brand-600 text-white font-bold text-xs rounded-lg transition-colors shrink-0 flex items-center justify-center gap-1.5 cursor-pointer uppercase tracking-wider"
              >
                <span>Join</span>
                <Send className="w-3.5 h-3.5" />
              </button>
            </form>
          </div>

        </div>
      </div>
    </Layout>
  );
}
