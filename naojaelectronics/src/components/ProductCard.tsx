import { Link } from 'react-router-dom';
import { Product } from '../types';

interface ProductCardProps {
  product: Product;
}

export default function ProductCard({ product }: ProductCardProps) {
  return (
    <Link
      to={`/product/${product.slug}`}
      className="group bg-white border border-gray-200 rounded-card overflow-hidden hover:shadow-hover transition-shadow"
    >
      <div className="h-44 bg-surface border-b border-gray-200 flex items-center justify-center">
        {product.image_url ? (
          <img
            src={product.image_url}
            alt={product.name}
            className="w-full h-full object-cover"
          />
        ) : (
          <div className="w-16 h-16 bg-gray-200 rounded-lg" />
        )}
      </div>

      <div className="p-3.5">
        <h3 className="font-bold text-gray-900 mb-1 line-clamp-2">{product.name}</h3>
        <p className="text-xs text-gray-500 mb-2.5 line-clamp-1">{product.brand}</p>
        <p className="font-extrabold text-lg text-gray-900 mb-3">
          KSh {product.price.toLocaleString()}
        </p>

        <span className="block w-full py-2.5 text-center border border-brand-500 text-brand-500 font-semibold rounded-control text-sm group-hover:bg-brand-50 transition-colors">
          View
        </span>
      </div>
    </Link>
  );
}
