import { useState, useEffect } from 'react';
import { useParams, Link } from 'react-router-dom';
import { ShoppingBag, Zap } from 'lucide-react';
import Layout from '../components/Layout';
import QuantityControl from '../components/QuantityControl';
import FulfillmentModule from '../components/FulfillmentModule';
import { useCart } from '../lib/cart';
import { supabase } from '../lib/supabase';
import { Product } from '../types';

const FALLBACK_PRODUCTS: Record<string, Product> = {
  'jbl-go-speaker': {
    id: 'jbl-1',
    name: 'JBL Go Portable Speaker',
    slug: 'jbl-go-speaker',
    description: 'Experience powerful sound with the JBL Go portable Bluetooth speaker. Compact design with deep bass and long battery life.',
    price: 4200,
    category_id: 'audio',
    category_name: 'Audio',
    brand: 'JBL',
    image_url: 'https://images.pexels.com/photos/1271911/pexels-photo-1271911.jpeg?auto=compress&cs=tinysrgb&w=800',
    in_stock: true,
    highlights: ['Powerful sound', 'Long battery life', 'Compact design', 'Bluetooth connectivity'],
    specifications: { 'Output': '5W', 'Connectivity': 'Bluetooth', 'Battery': '5 hours', 'Ports': 'USB-C' },
    created_at: new Date().toISOString(),
  },
  'oraimo-freepods-pro': {
    id: 'oraimo-1',
    name: 'Oraimo FreePods Pro',
    slug: 'oraimo-freepods-pro',
    description: 'Premium wireless earbuds with active noise cancellation and crystal clear sound.',
    price: 3999,
    category_id: 'audio',
    category_name: 'Audio',
    brand: 'Oraimo',
    image_url: 'https://images.pexels.com/photos/3780670/pexels-photo-3780670.jpeg?auto=compress&cs=tinysrgb&w=800',
    in_stock: true,
    highlights: ['Active Noise Cancellation', 'Bluetooth 5.2', '24hr Battery Life', 'Touch Controls'],
    specifications: { 'Driver Size': '10mm', 'Battery': '24hr total', 'Connectivity': 'Bluetooth 5.2', 'ANC': 'Yes' },
    created_at: new Date().toISOString(),
  },
  'vitron-43-smart-tv': {
    id: 'vitron-1',
    name: 'Vitron 43" Smart TV',
    slug: 'vitron-43-smart-tv',
    description: 'Full HD Smart TV with built-in streaming apps and stunning picture quality.',
    price: 18500,
    category_id: 'tvs-displays',
    category_name: 'TVs & Displays',
    brand: 'Vitron',
    image_url: 'https://images.pexels.com/photos/6937464/pexels-photo-6937464.jpeg?auto=compress&cs=tinysrgb&w=800',
    in_stock: true,
    highlights: ['Full HD 1080p', 'Smart TV Apps', 'HDMI x2', 'USB Playback'],
    specifications: { 'Screen Size': '43"', 'Resolution': '1080p Full HD', 'Smart Apps': 'Yes', 'HDMI Ports': '2' },
    created_at: new Date().toISOString(),
  },
  'laptop-16gb-512gb': {
    id: 'laptop-1',
    name: 'Laptop (16GB/512GB)',
    slug: 'laptop-16gb-512gb',
    description: 'Powerful laptop for work and study with fast processor and ample storage.',
    price: 55000,
    category_id: 'laptops-computers',
    category_name: 'Laptops & Computers',
    brand: 'Lenovo',
    image_url: 'https://images.pexels.com/photos/18105731/pexels-photo-18105731.jpeg?auto=compress&cs=tinysrgb&w=800',
    in_stock: true,
    highlights: ['16GB RAM', '512GB SSD', 'Fast Processor', '15.6" Display'],
    specifications: { 'RAM': '16GB', 'Storage': '512GB SSD', 'Display': '15.6"', 'Processor': 'Intel Core i5' },
    created_at: new Date().toISOString(),
  },
};

export default function ProductPage() {
  const { slug } = useParams();
  const { addItem } = useCart();
  const [product, setProduct] = useState<Product | null>(null);
  const [quantity, setQuantity] = useState(1);
  const [loading, setLoading] = useState(true);
  const [added, setAdded] = useState(false);

  useEffect(() => {
    async function fetchProduct() {
      setLoading(true);
      try {
        const { data } = await supabase
          .from('products')
          .select('*')
          .eq('slug', slug)
          .single();

        if (data) {
          setProduct(data);
        } else {
          setProduct(FALLBACK_PRODUCTS[slug || ''] || null);
        }
      } catch {
        setProduct(FALLBACK_PRODUCTS[slug || ''] || null);
      } finally {
        setLoading(false);
      }
    }

    fetchProduct();
  }, [slug]);

  const handleAddToCart = () => {
    if (product) {
      addItem(product, quantity);
      setAdded(true);
      setTimeout(() => setAdded(false), 2000);
    }
  };

  if (loading) {
    return (
      <Layout>
        <div className="max-w-7xl mx-auto px-4 py-8 text-center">Loading...</div>
      </Layout>
    );
  }

  if (!product) {
    return (
      <Layout>
        <div className="max-w-7xl mx-auto px-4 py-8 text-center">
          <p className="text-gray-500">Product not found</p>
          <Link to="/" className="text-brand-500 underline mt-2 inline-block">Back to home</Link>
        </div>
      </Layout>
    );
  }

  return (
    <Layout>
      <div className="max-w-7xl mx-auto px-4">
        {/* Breadcrumb */}
        <div className="text-xs text-gray-500 py-4">
          <Link to="/">Home</Link>
          <span className="mx-2">/</span>
          <Link to={`/category/${product.category_id}`}>{product.category_name}</Link>
          <span className="mx-2">/</span>
          <span className="text-gray-900">{product.name}</span>
        </div>

        {/* Product Detail */}
        <div className="grid lg:grid-cols-5 gap-6 py-8">
          {/* Gallery */}
          <div className="lg:col-span-3 h-[400px] lg:h-[520px] rounded-2xl border border-gray-200 bg-surface overflow-hidden">
            {product.image_url ? (
              <img
                src={product.image_url}
                alt={product.name}
                className="w-full h-full object-cover"
              />
            ) : (
              <div className="w-full h-full flex items-center justify-center text-gray-300">
                No image
              </div>
            )}
          </div>

          {/* Info */}
          <div className="lg:col-span-2">
            <div className="bg-white border border-gray-200 rounded-card p-4">
              {/* Stock */}
              <span className="inline-flex items-center bg-brand-100 text-brand-700 text-xs font-bold px-2.5 py-1 rounded-full border border-brand-200">
                {product.in_stock ? 'In stock' : 'Out of stock'}
              </span>

              {/* Title */}
              <h1 className="font-extrabold text-xl text-gray-900 mt-3 mb-2">{product.name}</h1>
              <p className="text-sm text-gray-500 mb-2.5">{product.brand}</p>

              {/* Price */}
              <p className="font-extrabold text-2xl text-gray-900 mb-3">
                KSh {product.price.toLocaleString()}
              </p>

              {/* Highlights */}
              <p className="text-xs text-gray-500 mb-3.5">
                <strong className="text-gray-900">Highlights:</strong>{' '}
                {product.highlights?.join(' - ') || 'Quality product from ' + product.brand}
              </p>

              {/* Quantity */}
              <div className="flex items-center gap-4 mb-3.5">
                <span className="text-xs text-gray-500">Quantity</span>
                <QuantityControl
                  quantity={quantity}
                  onIncrease={() => setQuantity((q) => q + 1)}
                  onDecrease={() => setQuantity((q) => Math.max(1, q - 1))}
                  size="sm"
                />
              </div>

              {/* Fulfillment Module */}
              <FulfillmentModule subtotal={product.price * quantity} />

              {/* Actions */}
              <div className="flex gap-3 mt-4">
                <button
                  onClick={handleAddToCart}
                  disabled={added}
                  className={`flex-1 h-11 rounded-control font-semibold flex items-center justify-center gap-2.5 transition-colors ${
                    added
                      ? 'bg-green-600 text-white'
                      : 'bg-brand-500 text-white hover:bg-brand-600'
                  }`}
                >
                  {added ? 'Added!' : (
                    <>
                      <ShoppingBag className="w-5 h-5" />
                      Add to Cart
                    </>
                  )}
                </button>
                <Link
                  to="/checkout"
                  className="flex-1 h-11 rounded-control font-semibold border border-brand-500 text-brand-500 flex items-center justify-center gap-2.5 hover:bg-brand-50 transition-colors"
                >
                  <Zap className="w-5 h-5" />
                  Buy Now
                </Link>
              </div>

              <p className="text-xs text-gray-500 mt-3">
                Secure checkout via <strong className="text-gray-900">M-Pesa</strong>. Warranty support available.
              </p>
            </div>

            {/* Specifications */}
            <div className="bg-white border border-gray-200 rounded-card p-4 mt-4">
              <h3 className="font-serif text-xl text-gray-900 mb-1.5">Specifications</h3>
              <div className="text-sm text-gray-500">
                {product.specifications && Object.keys(product.specifications).length > 0 ? (
                  Object.entries(product.specifications).map(([key, value]) => (
                    <div key={key} className="py-1">
                      <strong className="text-gray-700">{key}:</strong> {value}
                    </div>
                  ))
                ) : (
                  <p>Quality product from {product.brand}. Contact us for detailed specifications.</p>
                )}
              </div>
            </div>
          </div>
        </div>
      </div>
    </Layout>
  );
}
