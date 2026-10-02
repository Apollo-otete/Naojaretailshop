import React, { useState, useEffect } from 'react';
import {
  ResponsiveContainer,
  AreaChart,
  Area,
  XAxis,
  YAxis,
  Tooltip,
  CartesianGrid,
  Legend
} from 'recharts';
import { Target, TrendingUp, Sparkles, AlertTriangle, CheckCircle, PieChart, CreditCard, Edit3 } from 'lucide-react';
import { analyticsApi } from '../../lib/analyticsApi';

const formatKes = (amt) =>
  new Intl.NumberFormat('en-KE', { style: 'currency', currency: 'KES', maximumFractionDigits: 0 }).format(amt || 0);

export default function ForecastPlanningTab({ forecastData, insightsData, targetData, onRefresh }) {
  const [horizon, setHorizon] = useState(30);
  const [forecast, setForecast] = useState(forecastData?.forecast || []);
  const [projectedTotal, setProjectedTotal] = useState(forecastData?.projectedTotal || 0);
  const [growthRate, setGrowthRate] = useState(forecastData?.growthRate || 0);
  const [loadingForecast, setLoadingForecast] = useState(false);

  // Target modal / form
  const [isTargetModalOpen, setIsTargetModalOpen] = useState(false);
  const [targetForm, setTargetForm] = useState({
    period: 'month',
    revenueTarget: targetData?.revenueTarget || 600000,
    ordersTarget: targetData?.ordersTarget || 150
  });

  useEffect(() => {
    if (forecastData) {
      setForecast(forecastData.forecast || []);
      setProjectedTotal(forecastData.projectedTotal || 0);
      setGrowthRate(forecastData.growthRate || 0);
    }
  }, [forecastData]);

  const handleHorizonChange = async (days) => {
    setHorizon(days);
    setLoadingForecast(true);
    try {
      const data = await analyticsApi.getRevenueForecast(days);
      setForecast(data.forecast || []);
      setProjectedTotal(data.projectedTotal || 0);
      setGrowthRate(data.growthRate || 0);
    } catch (e) {
      console.warn('Forecast change error:', e.message);
    } finally {
      setLoadingForecast(false);
    }
  };

  const handleSaveTarget = async (e) => {
    e.preventDefault();
    try {
      const now = new Date();
      const periodStart = new Date(now.getFullYear(), now.getMonth(), 1).toISOString().split('T')[0];
      await analyticsApi.setTarget({
        period: targetForm.period,
        periodStart,
        revenueTarget: Number(targetForm.revenueTarget),
        ordersTarget: Number(targetForm.ordersTarget)
      });
      setIsTargetModalOpen(false);
      if (onRefresh) onRefresh();
    } catch (err) {
      alert('Failed to save target: ' + err.message);
    }
  };

  const getInsightIcon = (type) => {
    switch (type) {
      case 'positive':
        return <TrendingUp className="w-5 h-5 text-emerald-600" />;
      case 'critical':
      case 'warning':
        return <AlertTriangle className="w-5 h-5 text-amber-600" />;
      case 'info':
        return <PieChart className="w-5 h-5 text-blue-600" />;
      default:
        return <Sparkles className="w-5 h-5 text-purple-600" />;
    }
  };

  return (
    <div className="space-y-6">
      {/* Target Progress & Planning Header */}
      <div className="bg-gradient-to-r from-gray-900 to-slate-800 text-white rounded-2xl p-6 shadow-md relative overflow-hidden">
        <div className="flex flex-col md:flex-row md:items-center justify-between gap-4 mb-5">
          <div className="flex items-center gap-3">
            <div className="w-12 h-12 rounded-2xl bg-emerald-500/20 text-emerald-400 flex items-center justify-center shrink-0 border border-emerald-500/30">
              <Target className="w-6 h-6" />
            </div>
            <div>
              <div className="flex items-center gap-2">
                <h2 className="text-xl font-bold">Executive Targets & Milestone Tracker</h2>
                <span
                  className={`text-[10px] font-bold px-2.5 py-0.5 rounded-full uppercase tracking-wider ${
                    targetData?.status === 'surpassed'
                      ? 'bg-emerald-500/30 text-emerald-300 border border-emerald-500/50'
                      : targetData?.status === 'on_track'
                      ? 'bg-blue-500/30 text-blue-300 border border-blue-500/50'
                      : 'bg-amber-500/30 text-amber-300 border border-amber-500/50'
                  }`}
                >
                  {targetData?.status === 'surpassed'
                    ? 'Target Surpassed'
                    : targetData?.status === 'on_track'
                    ? 'On Track'
                    : 'Action Needed'}
                </span>
              </div>
              <p className="text-xs text-gray-300 mt-0.5">
                Current cycle target: {formatKes(targetData?.revenueTarget || 600000)} across {targetData?.ordersTarget || 150} orders
              </p>
            </div>
          </div>

          <button
            onClick={() => setIsTargetModalOpen(true)}
            className="inline-flex items-center gap-2 px-4 py-2 bg-brand-600 hover:bg-brand-500 text-white text-xs font-bold rounded-xl transition-all shadow-sm"
          >
            <Edit3 className="w-4 h-4" />
            Adjust Goals & Targets
          </button>
        </div>

        {/* Progress Bar */}
        <div className="space-y-2">
          <div className="flex justify-between text-xs font-semibold">
            <span className="text-gray-300">Revenue Achieved: {formatKes(targetData?.actualRevenue || 0)}</span>
            <span className="text-emerald-400 font-bold">{targetData?.revenueProgress || 0}% Complete</span>
          </div>
          <div className="w-full h-3 bg-gray-700/60 rounded-full overflow-hidden p-0.5">
            <div
              className="h-full bg-gradient-to-r from-emerald-500 to-teal-400 rounded-full transition-all duration-700"
              style={{ width: `${Math.min(100, targetData?.revenueProgress || 0)}%` }}
            ></div>
          </div>
        </div>
      </div>

      {/* Predictive Revenue Forecast Chart */}
      <div className="bg-white p-6 rounded-2xl border border-gray-100 shadow-sm">
        <div className="flex flex-col sm:flex-row sm:items-center justify-between gap-4 mb-6">
          <div>
            <div className="flex items-center gap-2">
              <h3 className="font-bold text-gray-900 text-base">Predictive Revenue Forecast (Confidence Interval)</h3>
              <span className="text-xs bg-purple-50 text-purple-700 font-bold px-2 py-0.5 rounded-md">
                Regression + Seasonality
              </span>
            </div>
            <p className="text-xs text-gray-400 mt-0.5">
              Projected gross sales based on 7-day moving averages and historical Kakamega retail cycle trends
            </p>
          </div>

          {/* Forecast Horizon Switcher */}
          <div className="inline-flex p-1 bg-gray-100 rounded-xl text-xs font-bold">
            {[
              { days: 7, label: 'Next 7 Days' },
              { days: 30, label: 'Next 30 Days' },
              { days: 90, label: 'Next 3 Months' },
              { days: 365, label: 'Next 1 Year' }
            ].map((btn) => (
              <button
                key={btn.days}
                onClick={() => handleHorizonChange(btn.days)}
                className={`px-3 py-1.5 rounded-lg transition-all ${
                  horizon === btn.days
                    ? 'bg-purple-600 text-white shadow-sm'
                    : 'text-gray-600 hover:text-gray-900'
                }`}
              >
                {btn.label}
              </button>
            ))}
          </div>
        </div>

        {/* Projected Highlights Summary */}
        <div className="grid grid-cols-2 sm:grid-cols-3 gap-3 mb-6 bg-purple-50/50 p-4 rounded-xl border border-purple-100">
          <div>
            <p className="text-[11px] font-semibold text-purple-900 uppercase">Projected Period Gross</p>
            <p className="text-xl font-extrabold text-purple-900 mt-0.5">{formatKes(projectedTotal)}</p>
          </div>
          <div>
            <p className="text-[11px] font-semibold text-purple-900 uppercase">Pace vs Previous Period</p>
            <p
              className={`text-xl font-extrabold mt-0.5 ${
                growthRate >= 0 ? 'text-emerald-700' : 'text-rose-700'
              }`}
            >
              {growthRate >= 0 ? '+' : ''}
              {growthRate}%
            </p>
          </div>
          <div className="col-span-2 sm:col-span-1">
            <p className="text-[11px] font-semibold text-purple-900 uppercase">Forecast Horizon</p>
            <p className="text-xl font-extrabold text-gray-800 mt-0.5">{horizon} Days Ahead</p>
          </div>
        </div>

        {/* Recharts Area Chart */}
        <div className="h-72 w-full">
          {forecast.length === 0 ? (
            <div className="h-full flex items-center justify-center text-sm text-gray-400">
              Generating predictive forecast curve...
            </div>
          ) : (
            <ResponsiveContainer width="100%" height="100%">
              <AreaChart data={forecast} margin={{ top: 10, right: 10, left: 0, bottom: 0 }}>
                <defs>
                  <linearGradient id="colorForecast" x1="0" y1="0" x2="0" y2="1">
                    <stop offset="5%" stopColor="#8b5cf6" stopOpacity={0.3} />
                    <stop offset="95%" stopColor="#8b5cf6" stopOpacity={0} />
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
                  tick={{ fontSize: 11, fill: '#64748b' }}
                  tickFormatter={(val) => (val >= 1000 ? `${val / 1000}k` : val)}
                  stroke="#cbd5e1"
                />
                <Tooltip
                  formatter={(val, name) => [
                    formatKes(val),
                    name === 'projected'
                      ? 'Projected Revenue'
                      : name === 'upperBound'
                      ? 'Upper Confidence Limit'
                      : 'Lower Confidence Limit'
                  ]}
                  contentStyle={{ backgroundColor: '#1e293b', borderRadius: '12px', color: '#fff', border: 'none' }}
                />
                <Area
                  type="monotone"
                  dataKey="upperBound"
                  stroke="#c4b5fd"
                  strokeDasharray="4 4"
                  fill="none"
                  name="upperBound"
                />
                <Area
                  type="monotone"
                  dataKey="projected"
                  stroke="#8b5cf6"
                  strokeWidth={2.5}
                  fillOpacity={1}
                  fill="url(#colorForecast)"
                  name="projected"
                />
                <Area
                  type="monotone"
                  dataKey="lowerBound"
                  stroke="#c4b5fd"
                  strokeDasharray="4 4"
                  fill="none"
                  name="lowerBound"
                />
              </AreaChart>
            </ResponsiveContainer>
          )}
        </div>
      </div>

      {/* Executive Insights Panel */}
      <div className="bg-white p-6 rounded-2xl border border-gray-100 shadow-sm">
        <div className="flex items-center gap-2 mb-4">
          <Sparkles className="w-5 h-5 text-emerald-600" />
          <h3 className="font-bold text-gray-900 text-base">Managerial Intelligence & Actionable Insights</h3>
        </div>

        <div className="grid grid-cols-1 md:grid-cols-2 gap-4">
          {(insightsData || []).map((insight, idx) => (
            <div
              key={idx}
              className={`p-4 rounded-xl border flex items-start gap-3.5 transition-all ${
                insight.type === 'positive'
                  ? 'bg-emerald-50/50 border-emerald-200'
                  : insight.type === 'critical'
                  ? 'bg-rose-50/50 border-rose-200'
                  : insight.type === 'warning'
                  ? 'bg-amber-50/50 border-amber-200'
                  : 'bg-blue-50/50 border-blue-200'
              }`}
            >
              <div className="w-9 h-9 rounded-lg bg-white shadow-sm flex items-center justify-center shrink-0">
                {getInsightIcon(insight.type)}
              </div>
              <div className="space-y-1">
                <h4 className="text-xs font-bold text-gray-900">{insight.title}</h4>
                <p className="text-xs text-gray-600 leading-relaxed">{insight.text}</p>
              </div>
            </div>
          ))}
        </div>
      </div>

      {/* Target Setting Modal */}
      {isTargetModalOpen && (
        <div className="fixed inset-0 z-50 bg-black/50 backdrop-blur-sm flex items-center justify-center p-4">
          <div className="bg-white rounded-2xl p-6 max-w-md w-full shadow-2xl space-y-4">
            <h3 className="text-lg font-bold text-gray-900">Configure Business Targets</h3>
            <p className="text-xs text-gray-500">
              Set realistic revenue and sales volume targets for your retail team in Lurambi.
            </p>

            <form onSubmit={handleSaveTarget} className="space-y-4 text-xs">
              <div>
                <label className="block font-semibold text-gray-700 mb-1">Target Cycle Period</label>
                <select
                  value={targetForm.period}
                  onChange={(e) => setTargetForm({ ...targetForm, period: e.target.value })}
                  className="w-full p-2.5 border rounded-xl bg-gray-50 focus:outline-none focus:border-emerald-500"
                >
                  <option value="week">Weekly</option>
                  <option value="month">Monthly</option>
                  <option value="year">Annual / Yearly</option>
                </select>
              </div>

              <div>
                <label className="block font-semibold text-gray-700 mb-1">Target Revenue (KES)</label>
                <input
                  type="number"
                  value={targetForm.revenueTarget}
                  onChange={(e) => setTargetForm({ ...targetForm, revenueTarget: e.target.value })}
                  className="w-full p-2.5 border rounded-xl bg-gray-50 focus:outline-none focus:border-emerald-500"
                  required
                />
              </div>

              <div>
                <label className="block font-semibold text-gray-700 mb-1">Target Total Orders</label>
                <input
                  type="number"
                  value={targetForm.ordersTarget}
                  onChange={(e) => setTargetForm({ ...targetForm, ordersTarget: e.target.value })}
                  className="w-full p-2.5 border rounded-xl bg-gray-50 focus:outline-none focus:border-emerald-500"
                  required
                />
              </div>

              <div className="flex justify-end gap-2 pt-3 border-t">
                <button
                  type="button"
                  onClick={() => setIsTargetModalOpen(false)}
                  className="px-4 py-2 border rounded-xl text-gray-700 hover:bg-gray-100"
                >
                  Cancel
                </button>
                <button
                  type="submit"
                  className="px-4 py-2 bg-brand-600 hover:bg-brand-700 text-white rounded-xl font-bold"
                >
                  Save Target
                </button>
              </div>
            </form>
          </div>
        </div>
      )}
    </div>
  );
}
