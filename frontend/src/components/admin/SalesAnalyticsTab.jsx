import React from 'react';
import {
  ResponsiveContainer,
  AreaChart,
  Area,
  BarChart,
  Bar,
  XAxis,
  YAxis,
  Tooltip,
  CartesianGrid,
  Legend,
  PieChart,
  Pie,
  Cell
} from 'recharts';
import { TrendingUp, ShoppingCart, Users, DollarSign, ArrowUpRight, ArrowDownRight } from 'lucide-react';
import KpiCard from './KpiCard';

const COLORS = ['#10b981', '#3b82f6', '#f59e0b', '#8b5cf6', '#ec4899', '#06b6d4', '#64748b'];

const formatKes = (amt) =>
  new Intl.NumberFormat('en-KE', { style: 'currency', currency: 'KES', maximumFractionDigits: 0 }).format(amt || 0);

export default function SalesAnalyticsTab({ summaryData, categoryData, range, setRange, loading }) {
  const summary = summaryData?.summary || {};
  const trend = summaryData?.revenueTrend || [];
  const categories = categoryData || [];

  return (
    <div className="space-y-6">
      {/* Top Range Selector & KPI Highlights */}
      <div className="flex flex-col sm:flex-row sm:items-center justify-between gap-4 bg-white p-4 rounded-2xl border border-gray-100 shadow-sm">
        <div>
          <h2 className="text-lg font-bold text-gray-900">Executive Sales & Revenue Intelligence</h2>
          <p className="text-xs text-gray-500">Live multi-period breakdown for business planning</p>
        </div>

        {/* Time-slicer Buttons: Week | Month | Year */}
        <div className="inline-flex p-1 bg-gray-100 rounded-xl">
          {['week', 'month', 'year'].map((r) => (
            <button
              key={r}
              onClick={() => setRange(r)}
              className={`px-4 py-1.5 text-xs font-bold rounded-lg transition-all capitalize ${
                range === r
                  ? 'bg-brand-600 text-white shadow-sm'
                  : 'text-gray-600 hover:text-gray-900 hover:bg-gray-200/60'
              }`}
            >
              {r === 'week' ? 'Last 7 Days' : r === 'month' ? 'Last 30 Days' : 'Last 12 Months'}
            </button>
          ))}
        </div>
      </div>

      {/* KPI Cards Row */}
      <div className="grid grid-cols-1 sm:grid-cols-2 lg:grid-cols-4 gap-4">
        <KpiCard
          title="Period Revenue"
          value={formatKes(summary.totalRevenue)}
          subtext="vs previous cycle"
          change={`${summary.revenueGrowth >= 0 ? '+' : ''}${summary.revenueGrowth || 0}%`}
          changeType={summary.revenueGrowth >= 0 ? 'positive' : 'negative'}
          icon={DollarSign}
          color="brand"
        />

        <KpiCard
          title="Paid Orders"
          value={`${summary.paidOrdersCount || 0} / ${summary.totalOrders || 0}`}
          subtext="Conversion rate"
          change={`${summary.conversionRate || 0}%`}
          changeType={summary.conversionRate >= 70 ? 'positive' : 'neutral'}
          icon={ShoppingCart}
          color="blue"
        />

        <KpiCard
          title="Average Order Value"
          value={formatKes(summary.aov)}
          subtext="Basket depth"
          change={`${summary.totalUnits || 0} units`}
          changeType="neutral"
          icon={TrendingUp}
          color="amber"
        />

        <KpiCard
          title="Active Customers"
          value={summary.uniqueCustomers || 0}
          subtext="Repeat buyers"
          change={`${summary.repeatCustomers || 0} returning`}
          changeType="positive"
          icon={Users}
          color="purple"
        />
      </div>

      {/* Main Revenue & Order Volume Chart */}
      <div className="bg-white p-6 rounded-2xl border border-gray-100 shadow-sm">
        <div className="flex flex-col sm:flex-row sm:items-center justify-between gap-2 mb-6">
          <div>
            <h3 className="font-bold text-gray-900 text-base">Revenue & Order Volume Trajectory</h3>
            <p className="text-xs text-gray-400">Daily gross revenue in KES and completed orders count</p>
          </div>
          <div className="flex items-center gap-4 text-xs font-semibold">
            <span className="flex items-center gap-1.5 text-emerald-600">
              <span className="w-3 h-3 rounded-full bg-emerald-500 inline-block"></span> Revenue (KES)
            </span>
            <span className="flex items-center gap-1.5 text-blue-600">
              <span className="w-3 h-3 rounded-full bg-blue-500 inline-block"></span> Order Count
            </span>
          </div>
        </div>

        <div className="h-72 w-full">
          {trend.length === 0 ? (
            <div className="h-full flex items-center justify-center text-sm text-gray-400">
              No sales data recorded in this timeframe.
            </div>
          ) : (
            <ResponsiveContainer width="100%" height="100%">
              <AreaChart data={trend} margin={{ top: 10, right: 10, left: 0, bottom: 0 }}>
                <defs>
                  <linearGradient id="colorRev" x1="0" y1="0" x2="0" y2="1">
                    <stop offset="5%" stopColor="#10b981" stopOpacity={0.25} />
                    <stop offset="95%" stopColor="#10b981" stopOpacity={0} />
                  </linearGradient>
                </defs>
                <CartesianGrid strokeDasharray="3 3" vertical={false} stroke="#f1f5f9" />
                <XAxis
                  dataKey="date"
                  tick={{ fontSize: 11, fill: '#64748b' }}
                  tickFormatter={(val) => {
                    const parts = val.split('-');
                    return parts.length >= 3 ? `${parts[1]}/${parts[2]}` : val;
                  }}
                  stroke="#cbd5e1"
                />
                <YAxis
                  yAxisId="left"
                  tick={{ fontSize: 11, fill: '#64748b' }}
                  tickFormatter={(val) => (val >= 1000 ? `${val / 1000}k` : val)}
                  stroke="#cbd5e1"
                />
                <YAxis
                  yAxisId="right"
                  orientation="right"
                  tick={{ fontSize: 11, fill: '#3b82f6' }}
                  stroke="#93c5fd"
                />
                <Tooltip
                  formatter={(value, name) => [
                    name === 'revenue' ? formatKes(value) : value,
                    name === 'revenue' ? 'Revenue' : 'Orders'
                  ]}
                  contentStyle={{ backgroundColor: '#1e293b', borderRadius: '12px', color: '#fff', border: 'none' }}
                />
                <Area
                  yAxisId="left"
                  type="monotone"
                  dataKey="revenue"
                  stroke="#10b981"
                  strokeWidth={2.5}
                  fillOpacity={1}
                  fill="url(#colorRev)"
                />
                <Bar yAxisId="right" dataKey="orders" fill="#3b82f6" radius={[4, 4, 0, 0]} barSize={14} />
              </AreaChart>
            </ResponsiveContainer>
          )}
        </div>
      </div>

      {/* Category Performance Section: Pie + Ranked Table */}
      <div className="grid grid-cols-1 lg:grid-cols-12 gap-6">
        {/* Pie / Share Chart */}
        <div className="lg:col-span-5 bg-white p-6 rounded-2xl border border-gray-100 shadow-sm flex flex-col justify-between">
          <div>
            <h3 className="font-bold text-gray-900 text-base mb-1">Department Revenue Share</h3>
            <p className="text-xs text-gray-400 mb-4">Contribution by product category</p>
          </div>

          <div className="h-64 flex items-center justify-center">
            {categories.length === 0 ? (
              <p className="text-xs text-gray-400">No category transactions available</p>
            ) : (
              <ResponsiveContainer width="100%" height="100%">
                <PieChart>
                  <Pie
                    data={categories}
                    dataKey="revenue"
                    nameKey="categoryName"
                    cx="50%"
                    cy="50%"
                    innerRadius={50}
                    outerRadius={80}
                    paddingAngle={3}
                  >
                    {categories.map((entry, index) => (
                      <Cell key={`cell-${index}`} fill={COLORS[index % COLORS.length]} />
                    ))}
                  </Pie>
                  <Tooltip
                    formatter={(val) => [formatKes(val), 'Revenue']}
                    contentStyle={{ backgroundColor: '#1e293b', borderRadius: '10px', color: '#fff', border: 'none' }}
                  />
                </PieChart>
              </ResponsiveContainer>
            )}
          </div>

          <div className="grid grid-cols-2 gap-2 mt-4 pt-4 border-t border-gray-100 text-xs">
            {categories.slice(0, 4).map((c, i) => (
              <div key={c.categoryId} className="flex items-center gap-2">
                <span className="w-2.5 h-2.5 rounded-full shrink-0" style={{ backgroundColor: COLORS[i % COLORS.length] }}></span>
                <span className="truncate text-gray-600 font-medium">{c.categoryName}</span>
                <span className="ml-auto font-bold text-gray-800">{c.shareOfRevenue}%</span>
              </div>
            ))}
          </div>
        </div>

        {/* Category Ranked Breakdown Table */}
        <div className="lg:col-span-7 bg-white p-6 rounded-2xl border border-gray-100 shadow-sm">
          <div className="flex items-center justify-between mb-4">
            <div>
              <h3 className="font-bold text-gray-900 text-base">Category Performance Leaderboard</h3>
              <p className="text-xs text-gray-400">Total volume and revenue per retail division</p>
            </div>
          </div>

          <div className="overflow-x-auto">
            <table className="w-full text-left text-xs">
              <thead>
                <tr className="border-b border-gray-100 text-gray-400 uppercase font-semibold">
                  <th className="pb-3">Category</th>
                  <th className="pb-3 text-center">Units Sold</th>
                  <th className="pb-3 text-right">Gross Revenue</th>
                  <th className="pb-3 text-right">Share</th>
                </tr>
              </thead>
              <tbody className="divide-y divide-gray-50">
                {categories.map((cat, idx) => (
                  <tr key={cat.categoryId} className="hover:bg-gray-50/70 transition-colors">
                    <td className="py-3 font-semibold text-gray-900 flex items-center gap-2">
                      <span>{cat.icon || '📦'}</span>
                      <span>{cat.categoryName}</span>
                    </td>
                    <td className="py-3 text-center text-gray-600 font-medium">{cat.unitsSold}</td>
                    <td className="py-3 text-right font-bold text-emerald-700">{formatKes(cat.revenue)}</td>
                    <td className="py-3 text-right">
                      <span className="inline-block px-2 py-0.5 rounded-full font-bold text-[11px] bg-emerald-50 text-emerald-700">
                        {cat.shareOfRevenue}%
                      </span>
                    </td>
                  </tr>
                ))}
              </tbody>
            </table>
          </div>
        </div>
      </div>
    </div>
  );
}
