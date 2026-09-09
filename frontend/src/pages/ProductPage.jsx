import { useState, useEffect } from 'react';
import { useParams, Link, useNavigate } from 'react-router-dom';
import { ShoppingBag, Zap, MessageSquare, ArrowLeft } from 'lucide-react';
import Layout from '../components/Layout';
import QuantityControl from '../components/QuantityControl';
import FulfillmentModule from '../components/FulfillmentModule';
import ProductCard from '../components/ProductCard';
import { useCart } from '../lib/cart';
import { api } from '../lib/api';

export default function ProductPage() {
  const { slug } = useParams();
  const navigate = useNavigate();
  const { addItem } = useCart();
  const [product, setProduct] = useState(null);
  const [relatedProducts, setRelatedProducts] = useState([]);
  const [quantity, setQuantity] = useState(1);
  const [loading, setLoading] = useState(true);
  const [added, setAdded] = useState(false);

  useEffect(() => {
    async function fetchProductData() {
      setLoading(true);
      try {
        const data = await api.getProductBySlug(slug);
        if (data) {
          setProduct(data);
          // Fetch related products in the same category
          const categorySlug = data.category_slug || data.category_id || '';
          if (categorySlug) {
            const allProducts = await api.getProducts(categorySlug);
            // Filter out current product and keep at most 4 related items
            const related = allProducts
              .filter((p) => p.id !== data.id)
              .slice(0, 4);
            setRelatedProducts(related);
          }
        }
      } catch (error) {
        console.error('Error fetching product details:', error);
      } finally {
        setLoading(false);
      }
    }

    fetchProductData();
    setQuantity(1); // Reset quantity on page change
  }, [slug]);

  const handleAddToCart = () => {
    if (product) {
      addItem(product, quantity);
      setAdded(true);
      setTimeout(() => setAdded(false), 2000);
    }
  };

  const handleBuyNow = () => {
    if (product) {
      addItem(product, quantity);
      navigate('/checkout');
    }
  };

  if (loading) {
    return (
      <Layout>
        <div className="max-w-7xl mx-auto px-4 py-16 text-center text-gray-500 font-medium">
          Loading product details...
        </div>
      </Layout>
    );
  }

  if (!product) {
    return (
      <Layout>
        <div className="max-w-7xl mx-auto px-4 py-16 text-center">
          <p className="text-gray-500 font-medium">Product not found</p>
          <Link to="/" className="text-brand-500 hover:underline mt-4 inline-block font-semibold">
            Back to Home
          </Link>
        </div>
      </Layout>
    );
  }

  // Construct WhatsApp inquiry link
  const encodedMessage = encodeURIComponent(
    `Hello Naoja Ventures, I am inquiring about the product: ${product.name} (Price: KSh ${product.price.toLocaleString()}). Is it currently in stock?`
  );
  const whatsappUrl = `https://wa.me/254704812343?text=${encodedMessage}`;

  return (
    <Layout>
      <div className="max-w-7xl mx-auto px-4 py-6">
        {/* Breadcrumb / Back Button */}
        <div className="flex items-center justify-between pb-6 border-b border-gray-100">
          <div className="text-xs text-gray-500">
            <Link to="/" className="hover:text-brand-500">Home</Link>
            <span className="mx-2">/</span>
            <Link to={`/category/${product.category_slug || product.category_id}`} className="hover:text-brand-500">
              {product.category_name || 'Category'}
            </Link>
            <span className="mx-2">/</span>
            <span className="text-gray-900">{product.name}</span>
          </div>
          <button 
            onClick={() => navigate(-1)} 
            className="flex items-center gap-1 text-xs font-semibold text-gray-500 hover:text-gray-900 cursor-pointer"
          >
            <ArrowLeft className="w-4 h-4" /> Back
          </button>
        </div>

        {/* Product Details Grid */}
        <div className="grid lg:grid-cols-5 gap-8 py-8">
          {/* Product Image */}
          <div className="lg:col-span-3 aspect-square max-h-[500px] lg:max-h-[520px] rounded-2xl border border-gray-200 bg-surface overflow-hidden flex items-center justify-center shadow-sm">
            {product.image_url ? (
              <img
                src={product.image_url}
                alt={product.name}
                className="w-full h-full object-cover"
              />
            ) : (
              <div className="w-full h-full flex items-center justify-center text-gray-400">
                No image available
              </div>
            )}
          </div>

          {/* Product Buying Info */}
          <div className="lg:col-span-2 space-y-6">
            <div className="bg-white border border-gray-200 rounded-card p-6 shadow-sm space-y-4">
              {/* Availability Badge */}
              <div>
                {product.in_stock === false || product.stock_status === 'Out of Stock' ? (
                  <span className="inline-flex items-center bg-red-50 text-red-700 text-xs font-bold px-2.5 py-1 rounded-full border border-red-200">
                    Out of stock
                  </span>
                ) : (
                  <span className="inline-flex items-center bg-brand-50 text-brand-700 text-xs font-bold px-2.5 py-1 rounded-full border border-brand-200">
                    In stock
                  </span>
                )}
              </div>

              {/* Title & Brand */}
              <div>
                <h1 className="font-serif text-2xl lg:text-3xl font-bold text-gray-900 leading-tight">
                  {product.name}
                </h1>
                <p className="text-xs text-gray-400 mt-1 uppercase font-semibold tracking-wider">
                  Brand: {product.brand || 'Generic'}
                </p>
              </div>

              {/* Price */}
              <div className="py-2 border-y border-gray-100">
                <p className="text-xs text-gray-400">Price</p>
                <p className="font-extrabold text-3xl text-gray-900 mt-0.5">
                  KSh {product.price.toLocaleString()}
                </p>
              </div>

              {/* Highlights */}
              {product.highlights && product.highlights.length > 0 && (
                <div>
                  <p className="text-xs font-bold uppercase text-gray-400 mb-2">Highlights</p>
                  <ul className="list-disc list-inside text-sm text-gray-600 space-y-1">
                    {product.highlights.map((h, i) => (
                      <li key={i}>{h}</li>
                    ))}
                  </ul>
                </div>
              )}

              {/* Description */}
              {product.description && (
                <div>
                  <p className="text-xs font-bold uppercase text-gray-400 mb-1">Description</p>
                  <p className="text-sm text-gray-600 leading-relaxed">{product.description}</p>
                </div>
              )}

              {/* Quantity Picker */}
              <div className="flex items-center gap-4 py-2">
                <span className="text-xs font-bold uppercase text-gray-400">Quantity</span>
                <QuantityControl
                  quantity={quantity}
                  onIncrease={() => setQuantity((q) => q + 1)}
                  onDecrease={() => setQuantity((q) => Math.max(1, q - 1))}
                  size="sm"
                />
              </div>

              {/* Fulfillment (delivery messages) */}
              <FulfillmentModule subtotal={product.price * quantity} />

              {/* Buy Actions */}
              <div className="space-y-2.5 pt-2">
                <div className="flex gap-3">
                  <button
                    onClick={handleAddToCart}
                    disabled={added || product.in_stock === false || product.stock_status === 'Out of Stock'}
                    className={`flex-1 h-11 rounded-control font-semibold flex items-center justify-center gap-2.5 transition-colors cursor-pointer ${
                      added
                        ? 'bg-green-600 text-white'
                        : 'bg-brand-500 text-white hover:bg-brand-600 disabled:bg-gray-100 disabled:text-gray-400 disabled:border-gray-200 disabled:cursor-not-allowed'
                    }`}
                  >
                    {added ? 'Added to Cart!' : (
                      <>
                        <ShoppingBag className="w-5 h-5" />
                        Add to Cart
                      </>
                    )}
                  </button>
                  <button
                    onClick={handleBuyNow}
                    disabled={product.in_stock === false || product.stock_status === 'Out of Stock'}
                    className="flex-1 h-11 rounded-control font-semibold border border-brand-500 text-brand-500 flex items-center justify-center gap-2.5 hover:bg-brand-50 transition-colors cursor-pointer disabled:border-gray-200 disabled:text-gray-400 disabled:cursor-not-allowed"
                  >
                    <Zap className="w-5 h-5" />
                    Buy Now
                  </button>
                </div>

                <a
                  href={whatsappUrl}
                  target="_blank"
                  rel="noopener noreferrer"
                  className="w-full h-11 bg-[#25D366] text-white font-semibold rounded-control inline-flex items-center justify-center gap-2.5 hover:bg-[#20ba56] transition-colors text-sm cursor-pointer"
                >
                  <MessageSquare className="w-5 h-5" />
                  Inquire via WhatsApp
                </a>
              </div>

              <p className="text-[11px] text-gray-400 text-center">
                Accepting payments via **M-Pesa Till 4149288**.
              </p>
            </div>

            {/* Specifications Box */}
            {product.specifications && Object.keys(product.specifications).length > 0 && (
              <div className="bg-white border border-gray-200 rounded-card p-6 shadow-sm">
                <h3 className="font-serif text-lg font-bold text-gray-900 mb-3 border-b border-gray-100 pb-2">
                  Technical Specifications
                </h3>
                <div className="text-sm space-y-2">
                  {Object.entries(product.specifications).map(([key, value]) => (
                    <div key={key} className="flex justify-between py-1 border-b border-gray-50 last:border-b-0">
                      <strong className="text-gray-600 font-medium">{key}</strong>
                      <span className="text-gray-500 text-right">{value}</span>
                    </div>
                  ))}
                </div>
              </div>
            )}
          </div>
        </div>

        {/* Related Products Section */}
        {relatedProducts.length > 0 && (
          <div className="py-12 border-t border-gray-100">
            <h2 className="font-serif text-2xl font-bold text-gray-900 mb-6">Related Products</h2>
            <div className="grid grid-cols-1 sm:grid-cols-2 lg:grid-cols-4 gap-6">
              {relatedProducts.map((p) => (
                <ProductCard key={p.id} product={p} />
              ))}
            </div>
          </div>
        )}
      </div>
    </Layout>
  );
}
