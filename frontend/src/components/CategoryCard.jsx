import { Link } from 'react-router-dom';
import { ArrowRight } from 'lucide-react';

export default function CategoryCard({ category }) {
  return (
    <Link
      to={`/category/${category.slug}`}
      className="group bg-white border border-gray-200 rounded-card overflow-hidden hover:shadow-hover transition-shadow"
    >
      <div className="h-32 bg-surface flex items-center justify-center overflow-hidden">
        {category.image_url ? (
          <img
            src={category.image_url}
            alt={category.name}
            className="w-full h-full object-cover group-hover:scale-105 transition-transform duration-300"
          />
        ) : (
          <div className="w-12 h-12 bg-gray-200 rounded-lg flex items-center justify-center">
            <span className="text-[10px] text-gray-400">No img</span>
          </div>
        )}
      </div>

      <div className="p-4">
        <p className="font-bold text-gray-900 group-hover:text-brand-500 transition-colors">
          {category.name}
        </p>
        {category.description && (
          <p className="text-xs text-gray-500 mt-1 line-clamp-2">{category.description}</p>
        )}
        <div className="flex items-center gap-1 mt-2 text-xs text-gray-400">
          <span>{category.product_count ?? 0} Products</span>
          <ArrowRight className="w-3 h-3 group-hover:translate-x-1 transition-transform" />
        </div>
      </div>
    </Link>
  );
}
