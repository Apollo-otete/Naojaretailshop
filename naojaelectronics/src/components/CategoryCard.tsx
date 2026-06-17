import { Link } from 'react-router-dom';
import { ArrowRight } from 'lucide-react';
import { Category } from '../types';

interface CategoryCardProps {
  category: Category;
}

export default function CategoryCard({ category }: CategoryCardProps) {
  return (
    <Link
      to={`/category/${category.slug}`}
      className="group bg-white border border-gray-200 rounded-card overflow-hidden hover:shadow-hover transition-shadow"
    >
      {/* Category Image */}
      <div className="h-32 bg-surface flex items-center justify-center overflow-hidden">
        {category.image_url ? (
          <img
            src={category.image_url}
            alt={category.name}
            className="w-full h-full object-cover group-hover:scale-105 transition-transform duration-300"
          />
        ) : (
          <div className="w-12 h-12 bg-gray-200 rounded-lg" />
        )}
      </div>

      {/* Category Info */}
      <div className="p-4">
        <p className="font-bold text-gray-900 group-hover:text-brand-500 transition-colors">
          {category.name}
        </p>
        {category.description && (
          <p className="text-xs text-gray-500 mt-1 line-clamp-2">{category.description}</p>
        )}
        <div className="flex items-center gap-1 mt-2 text-xs text-gray-400">
          <span>{category.product_count ?? 0} products</span>
          <ArrowRight className="w-3 h-3 group-hover:translate-x-1 transition-transform" />
        </div>
      </div>
    </Link>
  );
}
