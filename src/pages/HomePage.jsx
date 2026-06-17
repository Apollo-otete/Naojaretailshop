import { Link, useLocation } from 'react-router-dom';
import { useState, useEffect } from 'react';
import Layout from '../components/Layout';
import ProductCard from '../components/ProductCard';
import CategoryCard from '../components/CategoryCard';
import { api } from '../lib/api';
import { Star, Phone, MessageSquare, Shield, CheckCircle, Tag, Truck } from 'lucide-react';

const DEFAULT_CATEGORIES = [
  { id: '1', name: 'Phones', slug: 'phones', description: 'Smartphones & accessories', image_url: '', product_count: 45 },
  { id: '2', name: 'Laptops & Computers', slug: 'laptops-computers', description: 'Work & gaming', image_url: '', product_count: 32 },
  { id: '3', name: 'TVs & Displays', slug: 'tvs-displays', description: '4K, smart TVs', image_url: '', product_count: 28 },
  { id: '4', name: 'Audio', slug: 'audio', description: 'Speakers & earphones', image_url: '', product_count: 56 },
  { id: '5', name: 'Solar', slug: 'solar', description: 'Panels, inverters', image_url: '', product_count: 24 },
  { id: '6', name: 'Home Appliances', slug: 'home-appliances', description: 'Kitchen & home', image_url: '', product_count: 41 },
];

export default function HomePage() {
  const [categories, setCategories] = useState(DEFAULT_CATEGORIES);
  const [products, setProducts] = useState([]);
  const [reviews, setReviews] = useState([]);
  const [loading, setLoading] = useState(true);
  
  // Newsletter form state
  const [newsName, setNewsName] = useState('');
  const [newsEmail, setNewsEmail] = useState('');
  const [newsStatus, setNewsStatus] = useState({ success: false, message: '' });
  
  // Review form state
  const [revName, setRevName] = useState('');
  const [revRating, setRevRating] = useState(5);
  const [revComment, setRevComment] = useState('');
  const [revStatus, setRevStatus] = useState({ success: false, message: '' });

  const location = useLocation();
  const searchParams = new URLSearchParams(location.search);
  const search = searchParams.get('search') || '';

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
          setProducts(productsData.slice(0, 8)); // Display top 8 best sellers/newest
        }
        if (reviewsData) {
          setReviews(reviewsData);
        }
      } catch (error) {
        console.error('Error fetching data:', error);
      } finally {
        setLoading(false);
      }
    }

    fetchData();
  }, [search]);

  // Handle newsletter subscription
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

  // Handle review submission
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
        message: res.message || 'Review submitted! It will appear once approved by the administrator.' 
      });
      setRevName('');
      setRevComment('');
      setRevRating(5);
    } catch (error) {
      setRevStatus({ success: false, message: error.message || 'Failed to submit review.' });
    }
  };

  return (
    <Layout>
      {/* Hero Section */}
      <section className="relative bg-gradient-to-br from-brand-50/70 via-white to-gray-50 py-16 lg:py-24 border-b border-gray-100">
        <div className="max-w-7xl mx-auto px-4">
          <div className="grid lg:grid-cols-12 gap-8 items-center">
            <div className="lg:col-span-7 space-y-6">
              <span className="inline-flex items-center gap-1.5 px-3 py-1.5 rounded-full bg-brand-100 text-brand-800 text-xs font-semibold">
                <CheckCircle className="w-3.5 h-3.5 text-brand-600" />
                Genuine Electronics & Accessories
              </span>
              <h1 className="font-serif text-4xl lg:text-6xl font-bold leading-tight text-gray-900">
                Your Trusted Electrical & Electronics Partner in Kakamega
              </h1>
              <p className="text-gray-600 text-base lg:text-lg max-w-xl">
                Naoja Ventures offers a wide variety of genuine smartphones, computers, solar accessories, and home appliances at affordable prices. Pay conveniently via M-Pesa.
              </p>
              
              <div className="flex flex-wrap gap-3.5">
                <a href="#shop-products" className="btn-primary">
                  Shop Now
                </a>
                <Link to="/account?tab=support" className="btn-secondary">
                  Contact Us
                </Link>
                <a 
                  href="https://wa.me/254704812343" 
                  target="_blank" 
                  rel="noopener noreferrer" 
                  className="h-11 px-4 bg-[#25D366] text-white font-semibold rounded-control inline-flex items-center justify-center gap-2.5 hover:bg-[#20ba56] transition-colors"
                >
                  <MessageSquare className="w-5 h-5" />
                  WhatsApp Us
                </a>
              </div>

              <div className="flex flex-wrap gap-2 text-xs text-gray-500 pt-2">
                <span className="bg-white border border-gray-200 rounded-full px-3 py-1.5 font-medium">
                  📍 Lurambi, Kakamega (Opp. Bamboo)
                </span>
                <span className="bg-white border border-gray-200 rounded-full px-3 py-1.5 font-medium">
                  📞 0704812343
                </span>
                <span className="bg-white border border-gray-200 rounded-full px-3 py-1.5 font-medium">
                  🕒 Sun - Thu: 8:30 AM - 8:00 PM | Fri: 8:30 AM - 3:00 PM
                </span>
              </div>
            </div>

            {/* Custom visual element or logo showcase */}
            <div className="lg:col-span-5 flex justify-center">
              <div className="relative w-full max-w-md aspect-square bg-gradient-to-tr from-brand-100/50 to-brand-50 rounded-2xl border border-gray-200 flex flex-col items-center justify-center p-8 shadow-sm">
                <div className="w-28 h-28 bg-white border border-gray-200 rounded-full flex items-center justify-center shadow-sm mb-4">
                  <span className="font-serif text-3xl font-bold text-brand-600">Naoja</span>
                </div>
                <h3 className="font-serif text-xl font-bold text-gray-800">Naoja Ventures</h3>
                <p className="text-xs text-gray-400 mt-1 uppercase tracking-widest font-semibold">Kakamega, Kenya</p>
                <div className="mt-6 w-full bg-white border border-gray-100 rounded-xl p-4 space-y-2.5">
                  <div className="flex justify-between items-center text-xs">
                    <span className="text-gray-500">M-Pesa Buy Goods Till:</span>
                    <strong className="text-brand-600">4149288</strong>
                  </div>
                  <div className="h-px bg-gray-100"></div>
                  <div className="flex justify-between items-center text-xs">
                    <span className="text-gray-500">Fulfillment:</span>
                    <strong className="text-gray-800">Delivery or Pickup</strong>
                  </div>
                </div>
              </div>
            </div>
          </div>
        </div>
      </section>

      {/* Highlights Section */}
      <section className="py-12 bg-white border-b border-gray-100">
        <div className="max-w-7xl mx-auto px-4">
          <div className="grid grid-cols-1 sm:grid-cols-2 lg:grid-cols-5 gap-6">
            <div className="flex items-start gap-3.5 p-4 rounded-xl hover:bg-gray-50 transition-colors">
              <div className="p-2.5 bg-brand-50 text-brand-600 rounded-lg">
                <Shield className="w-6 h-6" />
              </div>
              <div>
                <h4 className="font-bold text-gray-900 text-sm">Genuine Products</h4>
                <p className="text-xs text-gray-500 mt-1">100% authentic devices & wiring from top brands.</p>
              </div>
            </div>

            <div className="flex items-start gap-3.5 p-4 rounded-xl hover:bg-gray-50 transition-colors">
              <div className="p-2.5 bg-brand-50 text-brand-600 rounded-lg">
                <Tag className="w-6 h-6" />
              </div>
              <div>
                <h4 className="font-bold text-gray-900 text-sm">Affordable Prices</h4>
                <p className="text-xs text-gray-500 mt-1">Kakamega's most competitive rates on all stock.</p>
              </div>
            </div>

            <div className="flex items-start gap-3.5 p-4 rounded-xl hover:bg-gray-50 transition-colors">
              <div className="p-2.5 bg-brand-50 text-brand-600 rounded-lg">
                <CheckCircle className="w-6 h-6" />
              </div>
              <div>
                <h4 className="font-bold text-gray-900 text-sm">Wide Variety</h4>
                <p className="text-xs text-gray-500 mt-1">From small bulb holders to smart TVs & panels.</p>
              </div>
            </div>

            <div className="flex items-start gap-3.5 p-4 rounded-xl hover:bg-gray-50 transition-colors">
              <div className="p-2.5 bg-brand-50 text-brand-600 rounded-lg">
                <Truck className="w-6 h-6" />
              </div>
              <div>
                <h4 className="font-bold text-gray-900 text-sm">Convenient Delivery</h4>
                <p className="text-xs text-gray-500 mt-1">Same-day Kakamega delivery for orders &ge; KSh 600.</p>
              </div>
            </div>

            <div className="flex items-start gap-3.5 p-4 rounded-xl hover:bg-gray-50 transition-colors">
              <div className="p-2.5 bg-brand-50 text-brand-600 rounded-lg">
                <Phone className="w-6 h-6" />
              </div>
              <div>
                <h4 className="font-bold text-gray-900 text-sm">Customer Support</h4>
                <p className="text-xs text-gray-500 mt-1">Dedicated helpline for enquiries and support.</p>
              </div>
            </div>
          </div>
        </div>
      </section>

      {/* Categories Section */}
      <section className="py-12 bg-gray-50/50">
        <div className="max-w-7xl mx-auto px-4">
          <div className="flex items-end justify-between mb-8">
            <div>
              <h2 className="font-serif text-3xl font-bold text-gray-900">Shop by Category</h2>
              <p className="text-sm text-gray-500 mt-1">Explore our wide selection of inventory categories.</p>
            </div>
          </div>
          
          <div className="grid grid-cols-2 sm:grid-cols-3 lg:grid-cols-6 gap-5">
            {categories.slice(0, 6).map((cat) => (
              <CategoryCard key={cat.id} category={cat} />
            ))}
          </div>
        </div>
      </section>

      {/* Products Section */}
      <section id="shop-products" className="py-16">
        <div className="max-w-7xl mx-auto px-4">
          <div className="mb-8">
            <h2 className="font-serif text-3xl font-bold text-gray-900">
              {search ? `Search Results for "${search}"` : 'Best Sellers'}
            </h2>
            <p className="text-sm text-gray-500 mt-1">
              {search ? `Found ${products.length} products` : 'Highly recommended products in Kakamega.'}
            </p>
          </div>

          {loading ? (
            <div className="text-center py-12 text-gray-500 font-medium">Loading products...</div>
          ) : products.length === 0 ? (
            <div className="text-center py-12 text-gray-500 border border-dashed border-gray-200 rounded-xl bg-white">
              No products found. Try a different search term or check categories.
            </div>
          ) : (
            <div className="grid grid-cols-1 sm:grid-cols-2 lg:grid-cols-4 gap-6">
              {products.map((product) => (
                <ProductCard key={product.id} product={product} />
              ))}
            </div>
          )}
        </div>
      </section>

      {/* Reviews & Testimonials Section */}
      <section className="py-16 bg-gray-50/70 border-y border-gray-100">
        <div className="max-w-7xl mx-auto px-4">
          <div className="grid lg:grid-cols-12 gap-12">
            {/* Reviews List */}
            <div className="lg:col-span-7">
              <h2 className="font-serif text-3xl font-bold text-gray-900 mb-2">Customer Testimonials</h2>
              <p className="text-sm text-gray-500 mb-8">What our clients in Kakamega say about Naoja Ventures.</p>

              {reviews.length === 0 ? (
                <div className="bg-white border border-gray-200 rounded-card p-6 text-center text-sm text-gray-500">
                  No testimonials available yet. Be the first to leave a review!
                </div>
              ) : (
                <div className="space-y-4">
                  {reviews.map((rev) => (
                    <div key={rev.id} className="bg-white border border-gray-200 rounded-card p-5 shadow-sm">
                      <div className="flex justify-between items-start mb-2">
                        <h4 className="font-bold text-gray-900 text-sm">{rev.name}</h4>
                        <div className="flex text-amber-400">
                          {[...Array(5)].map((_, i) => (
                            <Star 
                              key={i} 
                              className={`w-4 h-4 fill-current ${i < rev.rating ? 'text-amber-400' : 'text-gray-200'}`} 
                            />
                          ))}
                        </div>
                      </div>
                      <p className="text-xs text-gray-400 mb-3">{new Date(rev.created_at).toLocaleDateString()}</p>
                      <p className="text-sm text-gray-600 italic">"{rev.comment}"</p>
                    </div>
                  ))}
                </div>
              )}
            </div>

            {/* Leave a Review Form */}
            <div className="lg:col-span-5 bg-white border border-gray-200 rounded-card p-6 shadow-sm">
              <h3 className="font-serif text-xl font-bold text-gray-900 mb-2">Share Your Feedback</h3>
              <p className="text-xs text-gray-500 mb-4">Let us know how we can serve you better.</p>
              
              {revStatus.message && (
                <div className={`p-3.5 rounded-control text-xs font-semibold mb-4 ${
                  revStatus.success ? 'bg-green-50 text-green-700 border border-green-200' : 'bg-red-50 text-red-700 border border-red-200'
                }`}>
                  {revStatus.message}
                </div>
              )}

              <form onSubmit={handleReviewSubmit} className="space-y-4">
                <div>
                  <label className="block text-xs font-bold uppercase text-gray-500 mb-1.5">Your Name</label>
                  <input
                    type="text"
                    required
                    value={revName}
                    onChange={(e) => setRevName(e.target.value)}
                    placeholder="Enter your name"
                    className="w-full h-11 border border-gray-200 rounded-control px-3.5 text-sm focus:border-brand-500 outline-none"
                  />
                </div>

                <div>
                  <label className="block text-xs font-bold uppercase text-gray-500 mb-1.5">Rating (1-5 Stars)</label>
                  <div className="flex gap-2">
                    {[1, 2, 3, 4, 5].map((stars) => (
                      <button
                        key={stars}
                        type="button"
                        onClick={() => setRevRating(stars)}
                        className="p-1 hover:scale-110 transition-transform cursor-pointer"
                      >
                        <Star className={`w-7 h-7 fill-current ${stars <= revRating ? 'text-amber-400' : 'text-gray-200'}`} />
                      </button>
                    ))}
                  </div>
                </div>

                <div>
                  <label className="block text-xs font-bold uppercase text-gray-500 mb-1.5">Your Comment</label>
                  <textarea
                    required
                    rows={3}
                    value={revComment}
                    onChange={(e) => setRevComment(e.target.value)}
                    placeholder="Write your feedback..."
                    className="w-full border border-gray-200 rounded-control p-3.5 text-sm focus:border-brand-500 outline-none resize-none"
                  />
                </div>

                <button type="submit" className="btn-primary w-full text-sm">
                  Submit Review
                </button>
              </form>
            </div>
          </div>
        </div>
      </section>

      {/* Newsletter Subscription Section */}
      <section className="py-16 bg-white">
        <div className="max-w-4xl mx-auto px-4 text-center">
          <h2 className="font-serif text-3xl font-bold text-gray-900 mb-2">Subscribe to our Newsletter</h2>
          <p className="text-sm text-gray-500 mb-8 max-w-md mx-auto">
            Stay updated with new product arrivals, promotions, weekly deals, and electronics business updates from Kakamega.
          </p>

          {newsStatus.message && (
            <div className={`p-3.5 rounded-control text-xs font-semibold max-w-md mx-auto mb-6 ${
              newsStatus.success ? 'bg-green-50 text-green-700 border border-green-200' : 'bg-red-50 text-red-700 border border-red-200'
            }`}>
              {newsStatus.message}
            </div>
          )}

          <form onSubmit={handleNewsletterSubmit} className="flex flex-col sm:flex-row gap-3 max-w-xl mx-auto">
            <input
              type="text"
              required
              value={newsName}
              onChange={(e) => setNewsName(e.target.value)}
              placeholder="Your Name"
              className="flex-1 h-11 border border-gray-200 rounded-control px-4 text-sm focus:border-brand-500 outline-none"
            />
            <input
              type="email"
              required
              value={newsEmail}
              onChange={(e) => setNewsEmail(e.target.value)}
              placeholder="Your Email Address"
              className="flex-1 h-11 border border-gray-200 rounded-control px-4 text-sm focus:border-brand-500 outline-none"
            />
            <button type="submit" className="btn-primary h-11 px-6 text-sm shrink-0">
              Subscribe
            </button>
          </form>
        </div>
      </section>
    </Layout>
  );
}
