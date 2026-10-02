import React from 'react';
import { TrendingUp, TrendingDown, Minus } from 'lucide-react';

export default function KpiCard({
  title,
  value,
  subtext,
  change,
  changeType = 'neutral', // 'positive', 'negative', 'neutral'
  icon: Icon,
  color = 'brand', // 'brand', 'blue', 'amber', 'purple', 'rose'
  badge
}) {
  const colorStyles = {
    brand: {
      bg: 'bg-emerald-50',
      iconBg: 'bg-emerald-100 text-emerald-700',
      border: 'border-emerald-200/60',
      text: 'text-emerald-700'
    },
    blue: {
      bg: 'bg-blue-50',
      iconBg: 'bg-blue-100 text-blue-700',
      border: 'border-blue-200/60',
      text: 'text-blue-700'
    },
    amber: {
      bg: 'bg-amber-50',
      iconBg: 'bg-amber-100 text-amber-700',
      border: 'border-amber-200/60',
      text: 'text-amber-700'
    },
    orange: {
      bg: 'bg-orange-50',
      iconBg: 'bg-orange-100 text-[#ff6b00]',
      border: 'border-orange-200/60',
      text: 'text-[#ff6b00]'
    },
    purple: {
      bg: 'bg-purple-50',
      iconBg: 'bg-purple-100 text-purple-700',
      border: 'border-purple-200/60',
      text: 'text-purple-700'
    },
    rose: {
      bg: 'bg-rose-50',
      iconBg: 'bg-rose-100 text-rose-700',
      border: 'border-rose-200/60',
      text: 'text-rose-700'
    }
  };

  const style = colorStyles[color] || colorStyles.brand;

  return (
    <div className="naoja-kpi-card bg-white rounded-2xl p-5 border border-gray-100 shadow-sm relative overflow-hidden flex flex-col justify-between">
      {/* Top row */}
      <div className="flex items-center justify-between mb-3">
        <span className="text-xs font-semibold text-gray-500 uppercase tracking-wider">{title}</span>
        {Icon && (
          <div className={`w-10 h-10 rounded-xl flex items-center justify-center shrink-0 ${style.iconBg}`}>
            <Icon className="w-5 h-5" />
          </div>
        )}
      </div>

      {/* Main Value */}
      <div className="mb-2">
        <div className="text-2xl lg:text-3xl font-bold text-gray-900 tracking-tight">{value}</div>
      </div>

      {/* Footer / Trend */}
      <div className="flex items-center justify-between text-xs pt-1 border-t border-gray-50">
        <div className="flex items-center gap-1.5 font-medium">
          {change !== undefined && change !== null && (
            <span
              className={`inline-flex items-center px-1.5 py-0.5 rounded-full text-[11px] font-bold ${
                changeType === 'positive'
                  ? 'bg-emerald-100 text-emerald-800'
                  : changeType === 'negative'
                  ? 'bg-rose-100 text-rose-800'
                  : 'bg-gray-100 text-gray-700'
              }`}
            >
              {changeType === 'positive' ? (
                <TrendingUp className="w-3 h-3 mr-0.5" />
              ) : changeType === 'negative' ? (
                <TrendingDown className="w-3 h-3 mr-0.5" />
              ) : (
                <Minus className="w-3 h-3 mr-0.5" />
              )}
              {change}
            </span>
          )}
          {subtext && <span className="text-gray-400">{subtext}</span>}
        </div>
        {badge && (
          <span className="px-2 py-0.5 text-[10px] font-bold rounded-full bg-gray-100 text-gray-600">
            {badge}
          </span>
        )}
      </div>
    </div>
  );
}
