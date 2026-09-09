import { Link } from 'react-router-dom';

export default function ProductCard({ product }) {
  return (
    <Link
      to={`/product/${product.slug}`}
      className="group bg-white border border-gray-200 rounded-card overflow-hidden hover:shadow-hover transition-shadow flex flex-col h-full"
    >
      <div className="h-44 bg-surface border-b border-gray-200 flex items-center justify-center overflow-hidden">
        {product.image_url ? (
          <img
            src={product.image_url}
            alt={product.name}
            className="w-full h-full object-cover group-hover:scale-105 transition-transform duration-300"
          />
        ) : (
          <div className="w-16 h-16 bg-gray-200 rounded-lg flex items-center justify-center">
            <span className="text-xs text-gray-400">No image</span>
          </div>
        )}
      </div>

      <div className="p-3.5 flex flex-col flex-1">
        <h3 className="font-bold text-gray-900 mb-1 line-clamp-2 text-sm">{product.name}</h3>
        <p className="text-xs text-gray-400 mb-2.5 line-clamp-1">{product.brand}</p>
        <p className="font-extrabold text-base text-gray-900 mb-3 mt-auto">
          KSh {product.price.toLocaleString()}
        </p>

        {product.in_stock === false || product.stock_status === 'Out of Stock' ? (
          <span className="block w-full py-2 text-center bg-gray-100 text-gray-500 font-semibold rounded-control text-xs border border-gray-200">
            Out of Stock
          </span>
        ) : (
          <span className="block w-full py-2 text-center border border-brand-500 text-brand-500 font-semibold rounded-control text-xs group-hover:bg-brand-50 transition-colors">
            View Details
          </span>
        )}
      </div>
    </Link>
  );
}
