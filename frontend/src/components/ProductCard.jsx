import { Link } from 'react-router-dom';
import { ShoppingCart, Star, Check, Zap } from 'lucide-react';
import { useState } from 'react';
import { useCart } from '../lib/cart';

export default function ProductCard({ product, showDiscount = true }) {
  const { addItem } = useCart();
  const [added, setAdded] = useState(false);

  // Compute a realistic original price for visual discount display
  const discountPercent = product.discount_percent || 20;
  const originalPrice = Math.round(product.price * (1 + discountPercent / 100));

  const handleAddToCart = (e) => {
    e.preventDefault();
    e.stopPropagation();
    addItem(product, 1);
    setAdded(true);
    setTimeout(() => setAdded(false), 1500);
  };

  const isOutOfStock = product.in_stock === false || product.stock_status === 'Out of Stock' || product.stock_quantity <= 0;

  return (
    <div className="group relative bg-white border border-gray-200/80 rounded-xl overflow-hidden hover:shadow-lg hover:border-brand-300/60 transition-all duration-300 flex flex-col h-full">
      
      {/* Clickable Image Section */}
      <Link to={`/product/${product.slug}`} className="block relative overflow-hidden bg-gray-50 aspect-square">
        {product.image_url || (product.images && product.images[0]) ? (
          <img
            src={product.image_url || product.images[0]}
            alt={product.name}
            loading="lazy"
            className="w-full h-full object-cover object-center group-hover:scale-105 transition-transform duration-500"
          />
        ) : (
          <div className="w-full h-full flex items-center justify-center text-gray-300 text-xs font-semibold">
            No Image
          </div>
        )}

        {/* Discount Badge */}
        {showDiscount && !isOutOfStock && (
          <div className="absolute top-2 left-2 bg-retail-red text-white text-[10px] font-black px-1.5 py-0.5 rounded shadow-sm">
            -{discountPercent}%
          </div>
        )}

        {/* Naoja Direct Dispatch Badge */}
        {!isOutOfStock && (
          <div className="absolute bottom-2 left-2 bg-white/95 backdrop-blur-sm text-brand-700 text-[9px] font-extrabold px-1.5 py-0.5 rounded shadow-sm border border-brand-100 flex items-center gap-0.5">
            <Zap className="w-2.5 h-2.5 fill-brand-600 text-brand-600" />
            <span>Naoja Direct</span>
          </div>
        )}

        {isOutOfStock && (
          <div className="absolute inset-0 bg-black/40 backdrop-blur-[1px] flex items-center justify-center">
            <span className="bg-gray-900 text-white font-black text-[10px] px-2.5 py-1 rounded-full uppercase tracking-wider">
              Out of Stock
            </span>
          </div>
        )}
      </Link>

      {/* Product Details Section */}
      <div className="p-3 flex flex-col flex-1">
        
        {/* Title */}
        <Link 
          to={`/product/${product.slug}`}
          className="font-semibold text-gray-900 text-xs leading-snug line-clamp-2 h-8 hover:text-brand-600 transition-colors mb-1.5"
          title={product.name}
        >
          {product.name}
        </Link>

        {/* Star Rating */}
        <div className="flex items-center gap-1 mb-2">
          <div className="flex gap-px">
            {[1,2,3,4,5].map((s) => (
              <Star key={s} className={`w-3 h-3 ${s <= Math.round(product.rating_avg || 4.5) ? 'fill-amber-400 text-amber-400' : 'fill-gray-200 text-gray-200'}`} />
            ))}
          </div>
          <span className="text-[10px] text-gray-400">({product.rating_count || 12})</span>
        </div>

        {/* Pricing */}
        <div className="mt-auto">
          <div className="flex items-baseline gap-1.5 mb-2">
            <span className="text-base font-black text-brand-600">
              KSh {product.price.toLocaleString()}
            </span>
            <span className="text-[10px] text-gray-400 line-through">
              KSh {originalPrice.toLocaleString()}
            </span>
          </div>

          {/* Quick Action Button */}
          {isOutOfStock ? (
            <button
              disabled
              className="w-full min-h-[42px] py-2 bg-gray-100 text-gray-400 text-[11px] font-bold rounded-lg cursor-not-allowed touch-manipulation"
            >
              Sold Out
            </button>
          ) : (
            <button
              onClick={handleAddToCart}
              className={`w-full min-h-[42px] py-2 px-2 rounded-lg text-[11px] font-bold transition-all duration-200 flex items-center justify-center gap-1.5 active:scale-95 cursor-pointer touch-manipulation shadow-xs ${
                added
                  ? 'bg-emerald-600 text-white'
                  : 'bg-brand-500 hover:bg-brand-600 text-white'
              }`}
            >
              {added ? (
                <>
                  <Check className="w-3.5 h-3.5" />
                  <span>Added!</span>
                </>
              ) : (
                <>
                  <ShoppingCart className="w-3.5 h-3.5" />
                  <span>Add to Cart</span>
                </>
              )}
            </button>
          )}
        </div>
      </div>
    </div>
  );
}
