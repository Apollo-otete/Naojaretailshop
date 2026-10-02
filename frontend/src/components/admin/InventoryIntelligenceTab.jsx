import React, { useState } from 'react';
import { Download, Search, AlertCircle, CheckCircle2, ShieldAlert, Clock, ArrowDownCircle } from 'lucide-react';
import { analyticsApi } from '../../lib/analyticsApi';

export default function InventoryIntelligenceTab({ inventoryData, loading }) {
  const [filter, setFilter] = useState('all');
  const [search, setSearch] = useState('');

  const items = inventoryData || [];

  const filteredItems = items.filter((item) => {
    const matchesSearch = item.name.toLowerCase().includes(search.toLowerCase());
    if (filter === 'all') return matchesSearch;
    if (filter === 'low') return matchesSearch && (item.status === 'low' || item.status === 'critical');
    if (filter === 'critical') return matchesSearch && item.status === 'critical';
    if (filter === 'out') return matchesSearch && item.status === 'out_of_stock';
    if (filter === 'dead') return matchesSearch && item.status === 'dead_stock';
    if (filter === 'healthy') return matchesSearch && item.status === 'healthy';
    return matchesSearch;
  });

  const getStatusBadge = (status) => {
    switch (status) {
      case 'critical':
        return (
          <span className="inline-flex items-center gap-1 px-2.5 py-1 rounded-full text-xs font-bold bg-rose-100 text-rose-800 border border-rose-200">
            <ShieldAlert className="w-3.5 h-3.5 text-rose-600" /> Critical Stock
          </span>
        );
      case 'low':
        return (
          <span className="inline-flex items-center gap-1 px-2.5 py-1 rounded-full text-xs font-bold bg-amber-100 text-amber-800 border border-amber-200">
            <AlertCircle className="w-3.5 h-3.5 text-amber-600" /> Low Stock
          </span>
        );
      case 'out_of_stock':
        return (
          <span className="inline-flex items-center gap-1 px-2.5 py-1 rounded-full text-xs font-bold bg-gray-900 text-white">
            <ArrowDownCircle className="w-3.5 h-3.5 text-gray-400" /> Out of Stock
          </span>
        );
      case 'dead_stock':
        return (
          <span className="inline-flex items-center gap-1 px-2.5 py-1 rounded-full text-xs font-semibold bg-gray-100 text-gray-700">
            <Clock className="w-3.5 h-3.5 text-gray-500" /> Dead Stock
          </span>
        );
      default:
        return (
          <span className="inline-flex items-center gap-1 px-2.5 py-1 rounded-full text-xs font-bold bg-emerald-100 text-emerald-800 border border-emerald-200">
            <CheckCircle2 className="w-3.5 h-3.5 text-emerald-600" /> Healthy
          </span>
        );
    }
  };

  const handleDownloadPO = () => {
    const url = analyticsApi.getExportUrl('inventory');
    window.open(url, '_blank');
  };

  const lowCount = items.filter((i) => i.status === 'low' || i.status === 'critical').length;
  const outCount = items.filter((i) => i.status === 'out_of_stock').length;

  return (
    <div className="space-y-6">
      {/* Top Controls & Metrics */}
      <div className="bg-white p-5 rounded-2xl border border-gray-100 shadow-sm flex flex-col md:flex-row md:items-center justify-between gap-4">
        <div>
          <h2 className="text-lg font-bold text-gray-900">Inventory Velocity & Depletion Intelligence</h2>
          <p className="text-xs text-gray-500">
            Real-time run-out predictions and smart reorder suggestions based on daily sales velocity.
          </p>
        </div>

        <button
          onClick={handleDownloadPO}
          className="inline-flex items-center gap-2 px-4 py-2 bg-brand-600 hover:bg-brand-700 text-white text-xs font-bold rounded-xl shadow-sm transition-all"
        >
          <Download className="w-4 h-4" />
          Export Purchase Order (CSV)
        </button>
      </div>

      {/* Summary Alert Banner if low stock */}
      {(lowCount > 0 || outCount > 0) && (
        <div className="bg-amber-50 border border-amber-200 rounded-2xl p-4 flex items-center justify-between">
          <div className="flex items-center gap-3">
            <div className="w-10 h-10 rounded-xl bg-amber-100 text-amber-700 flex items-center justify-center shrink-0 font-bold">
              ⚠️
            </div>
            <div>
              <p className="text-sm font-bold text-amber-900">
                Action Required: {lowCount + outCount} items need warehouse restock
              </p>
              <p className="text-xs text-amber-700">
                {outCount} items are completely out of stock and {lowCount} are approaching safe buffer limits.
              </p>
            </div>
          </div>
          <button
            onClick={() => setFilter('low')}
            className="text-xs font-bold text-amber-800 underline hover:text-amber-900 shrink-0"
          >
            Filter Critical Items
          </button>
        </div>
      )}

      {/* Filter and Search Bar */}
      <div className="bg-white p-4 rounded-2xl border border-gray-100 shadow-sm flex flex-col sm:flex-row sm:items-center justify-between gap-3">
        <div className="relative flex-1 max-w-md">
          <Search className="w-4 h-4 absolute left-3.5 top-1/2 -translate-y-1/2 text-gray-400" />
          <input
            type="text"
            placeholder="Search products by title or model..."
            value={search}
            onChange={(e) => setSearch(e.target.value)}
            className="w-full pl-10 pr-4 py-2 text-xs bg-gray-50 border border-gray-200 rounded-xl focus:outline-none focus:border-emerald-500"
          />
        </div>

        {/* Filter Pills */}
        <div className="flex flex-wrap items-center gap-1.5 text-xs">
          {[
            { id: 'all', label: `All (${items.length})` },
            { id: 'low', label: `Low / Critical (${lowCount})` },
            { id: 'out', label: `Out of Stock (${outCount})` },
            { id: 'healthy', label: 'Healthy' }
          ].map((tab) => (
            <button
              key={tab.id}
              onClick={() => setFilter(tab.id)}
              className={`px-3 py-1.5 rounded-lg font-bold transition-all ${
                filter === tab.id
                  ? 'bg-gray-900 text-white'
                  : 'bg-gray-100 text-gray-600 hover:bg-gray-200'
              }`}
            >
              {tab.label}
            </button>
          ))}
        </div>
      </div>

      {/* Main Inventory Intelligence Table */}
      <div className="bg-white rounded-2xl border border-gray-100 shadow-sm overflow-hidden">
        <div className="overflow-x-auto">
          <table className="w-full text-left text-xs">
            <thead>
              <tr className="bg-gray-50/80 border-b border-gray-200 text-gray-500 uppercase font-semibold">
                <th className="py-3 px-4">Product Details</th>
                <th className="py-3 px-4 text-center">In Stock</th>
                <th className="py-3 px-4 text-center">Daily Velocity</th>
                <th className="py-3 px-4 text-center">Days Remaining</th>
                <th className="py-3 px-4 text-center">Predicted Stock-Out</th>
                <th className="py-3 px-4 text-center">Health Status</th>
                <th className="py-3 px-4 text-right">Suggested Reorder</th>
              </tr>
            </thead>
            <tbody className="divide-y divide-gray-100">
              {filteredItems.length === 0 ? (
                <tr>
                  <td colSpan={7} className="py-8 text-center text-gray-400">
                    No products matching your search criteria.
                  </td>
                </tr>
              ) : (
                filteredItems.map((item) => (
                  <tr key={item.productId} className="hover:bg-gray-50/70 transition-colors">
                    {/* Name */}
                    <td className="py-3.5 px-4">
                      <div className="font-bold text-gray-900 line-clamp-1">{item.name}</div>
                      <div className="text-[10px] text-gray-400">ID: #{item.productId}</div>
                    </td>

                    {/* Current Stock */}
                    <td className="py-3.5 px-4 text-center">
                      <span
                        className={`font-extrabold text-sm ${
                          item.currentStock <= 2
                            ? 'text-rose-600'
                            : item.currentStock <= 5
                            ? 'text-amber-600'
                            : 'text-gray-900'
                        }`}
                      >
                        {item.currentStock}
                      </span>
                    </td>

                    {/* Velocity */}
                    <td className="py-3.5 px-4 text-center">
                      <span className="font-semibold text-gray-700">{item.dailyVelocity}</span>
                      <span className="text-[10px] text-gray-400 ml-1">units/day</span>
                    </td>

                    {/* Days Remaining */}
                    <td className="py-3.5 px-4 text-center">
                      <span
                        className={`font-bold ${
                          item.daysRemaining <= 5
                            ? 'text-rose-600'
                            : item.daysRemaining <= 14
                            ? 'text-amber-600'
                            : 'text-emerald-700'
                        }`}
                      >
                        {item.daysRemaining >= 900 ? '90+ days' : `${item.daysRemaining} days`}
                      </span>
                    </td>

                    {/* Predicted Stockout Date */}
                    <td className="py-3.5 px-4 text-center text-gray-500 font-medium">
                      {item.predictedStockoutDate ? item.predictedStockoutDate : 'Stable'}
                    </td>

                    {/* Status Badge */}
                    <td className="py-3.5 px-4 text-center">{getStatusBadge(item.status)}</td>

                    {/* Suggested Reorder */}
                    <td className="py-3.5 px-4 text-right">
                      {item.suggestedReorderQty > 0 ? (
                        <span className="font-bold text-emerald-700 bg-emerald-50 px-2 py-1 rounded-md">
                          +{item.suggestedReorderQty} units
                        </span>
                      ) : (
                        <span className="text-gray-400">—</span>
                      )}
                    </td>
                  </tr>
                ))
              )}
            </tbody>
          </table>
        </div>
      </div>
    </div>
  );
}
