import React from 'react';
import { Truck, Store, Clock, CheckCircle2, ArrowRight } from 'lucide-react';
import KpiCard from './KpiCard';

export default function FulfillmentTab({ fulfillmentData, range, setRange }) {
  const data = fulfillmentData || {};
  const funnel = data.funnel || [];
  const statusCounts = data.statusBreakdown || {};
  const deliverySplit = data.deliverySplit || { pickup: 0, homeDelivery: 0, pickupPct: 50 };

  return (
    <div className="space-y-6">
      {/* Header & Range */}
      <div className="flex flex-col sm:flex-row sm:items-center justify-between gap-4 bg-white p-5 rounded-2xl border border-gray-100 shadow-sm">
        <div>
          <h2 className="text-lg font-bold text-gray-900">Order Fulfillment & Logistics Funnel</h2>
          <p className="text-xs text-gray-500">
            Dispatch speed, dispatch bottlenecks, and Storefront Pickup vs Kakamega Town deliveries
          </p>
        </div>

        <div className="inline-flex p-1 bg-gray-100 rounded-xl">
          {['week', 'month', 'year'].map((r) => (
            <button
              key={r}
              onClick={() => setRange(r)}
              className={`px-4 py-1.5 text-xs font-bold rounded-lg transition-all capitalize ${
                range === r
                  ? 'bg-brand-600 text-white shadow-sm'
                  : 'text-gray-600 hover:text-gray-900'
              }`}
            >
              {r === 'week' ? 'Last 7 Days' : r === 'month' ? 'Last 30 Days' : 'Last 12 Months'}
            </button>
          ))}
        </div>
      </div>

      {/* KPI Cards */}
      <div className="grid grid-cols-1 sm:grid-cols-2 lg:grid-cols-4 gap-4">
        <KpiCard
          title="Avg Dispatch Turnaround"
          value={`${data.avgFulfillmentHours || 3.5}h`}
          subtext="Same-day dispatch"
          change="Fast Kakamega delivery"
          changeType="positive"
          icon={Clock}
          color="brand"
        />

        <KpiCard
          title="Store Pickups"
          value={deliverySplit.pickup}
          subtext="Lurambi retail shop"
          change={`${deliverySplit.pickupPct}% of total`}
          changeType="neutral"
          icon={Store}
          color="blue"
        />

        <KpiCard
          title="Home Deliveries"
          value={deliverySplit.homeDelivery}
          subtext="Kakamega region"
          change={`${100 - deliverySplit.pickupPct}% of total`}
          changeType="positive"
          icon={Truck}
          color="purple"
        />

        <KpiCard
          title="Delivered Successfully"
          value={statusCounts.delivered || 0}
          subtext="Completed lifecycle"
          change="Zero return disputes"
          changeType="positive"
          icon={CheckCircle2}
          color="brand"
        />
      </div>

      {/* Order Fulfillment Funnel */}
      <div className="bg-white p-6 rounded-2xl border border-gray-100 shadow-sm space-y-4">
        <div>
          <h3 className="font-bold text-gray-900 text-base">Order Processing Pipeline</h3>
          <p className="text-xs text-gray-400">Step-by-step conversion from checkout to doorstep</p>
        </div>

        <div className="grid grid-cols-1 md:grid-cols-4 gap-4 pt-2">
          {funnel.map((step, idx) => (
            <div
              key={idx}
              className="bg-gray-50/70 border border-gray-100 p-4 rounded-xl relative overflow-hidden flex flex-col justify-between"
            >
              <div>
                <span className="text-[10px] font-bold text-gray-400 uppercase tracking-wider">
                  Stage {idx + 1}
                </span>
                <h4 className="text-sm font-bold text-gray-900 mt-1">{step.stage}</h4>
              </div>

              <div className="mt-4 pt-3 border-t border-gray-200/50 flex items-center justify-between">
                <span className="text-2xl font-extrabold text-emerald-800">{step.count}</span>
                {idx < funnel.length - 1 && (
                  <ArrowRight className="w-4 h-4 text-gray-400 hidden md:block" />
                )}
              </div>
            </div>
          ))}
        </div>
      </div>
    </div>
  );
}
