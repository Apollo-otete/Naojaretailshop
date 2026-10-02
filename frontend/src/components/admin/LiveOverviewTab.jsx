import React, { useState } from 'react';
import {
  Activity,
  DollarSign,
  ShoppingCart,
  Smartphone,
  AlertTriangle,
  Radio,
  Volume2,
  VolumeX,
  Clock,
  ShieldCheck,
  CheckCircle,
  Package
} from 'lucide-react';
import KpiCard from './KpiCard';

const formatKes = (amt) =>
  new Intl.NumberFormat('en-KE', { style: 'currency', currency: 'KES', maximumFractionDigits: 0 }).format(amt || 0);

export default function LiveOverviewTab({ liveData, events, soundEnabled, setSoundEnabled }) {
  const live = liveData || {};

  const getEventBadge = (type) => {
    switch (type) {
      case 'payment_received':
        return <span className="px-2 py-0.5 rounded-full text-[10px] font-bold bg-emerald-100 text-emerald-800">Payment Paid</span>;
      case 'order_created':
        return <span className="px-2 py-0.5 rounded-full text-[10px] font-bold bg-blue-100 text-blue-800">New Order</span>;
      case 'payment_failed':
        return <span className="px-2 py-0.5 rounded-full text-[10px] font-bold bg-rose-100 text-rose-800">Failed M-Pesa</span>;
      case 'low_stock':
        return <span className="px-2 py-0.5 rounded-full text-[10px] font-bold bg-amber-100 text-amber-800">Low Stock</span>;
      default:
        return <span className="px-2 py-0.5 rounded-full text-[10px] font-bold bg-gray-100 text-gray-800">System Event</span>;
    }
  };

  return (
    <div className="space-y-6">
      {/* Live Stream Status Banner */}
      <div className="bg-gradient-to-r from-emerald-950 via-gray-900 to-gray-900 text-white p-5 rounded-2xl border border-emerald-900/40 shadow-sm flex flex-col sm:flex-row sm:items-center justify-between gap-4">
        <div className="flex items-center gap-3">
          <div className="relative flex h-4 w-4">
            <span className="animate-ping absolute inline-flex h-full w-full rounded-full bg-emerald-400 opacity-75"></span>
            <span className="relative inline-flex rounded-full h-4 w-4 bg-emerald-500"></span>
          </div>
          <div>
            <div className="flex items-center gap-2">
              <h2 className="text-base font-bold text-white tracking-wide">Naoja Live Command Centre</h2>
              <span className="text-[10px] font-extrabold uppercase px-2 py-0.5 rounded-md bg-emerald-500/20 text-emerald-400 border border-emerald-500/30">
                REAL-TIME SSE BUS ACTIVE
              </span>
            </div>
            <p className="text-xs text-gray-400 mt-0.5">
              Listening to storefront checkouts, Daraja M-Pesa instant receipts, and warehouse stock
            </p>
          </div>
        </div>

        {/* Audio notification toggle */}
        <button
          onClick={() => setSoundEnabled(!soundEnabled)}
          className={`inline-flex items-center gap-2 px-3.5 py-1.5 rounded-xl text-xs font-bold transition-all ${soundEnabled
            ? 'bg-emerald-600/40 text-emerald-300 border border-emerald-500/40'
            : 'bg-gray-800 text-gray-400 hover:text-white'
            }`}
        >
          {soundEnabled ? <Volume2 className="w-4 h-4" /> : <VolumeX className="w-4 h-4" />}
          <span>{soundEnabled ? 'Live Audio Alerts ON' : 'Audio Muted'}</span>
        </button>
      </div>

      {/* Real-Time KPIs */}
      <div className="grid grid-cols-1 sm:grid-cols-2 lg:grid-cols-4 gap-4">
        <KpiCard
          title="Today's Gross Sales"
          value={formatKes(live.revenueToday || 0)}
          subtext="Cleared M-Pesa"
          change={`${live.paidOrdersToday || 0} completed`}
          changeType="positive"
          icon={DollarSign}
          color="brand"
        />

        <KpiCard
          title="Orders Placed Today"
          value={live.ordersToday || 0}
          subtext="Activity"
          change={`${live.ordersLast15Min || 0} in last 15 min`}
          changeType={live.ordersLast15Min > 0 ? 'positive' : 'neutral'}
          icon={ShoppingCart}
          color="blue"
        />

        <KpiCard
          title="In-Flight STK Prompts"
          value={live.inFlightStkPushes || 0}
          subtext="Checkout in progress"
          change="Pending PIN entry"
          changeType={live.inFlightStkPushes > 0 ? 'positive' : 'neutral'}
          icon={Smartphone}
          color="purple"
        />

        <KpiCard
          title="Inventory Alerts"
          value={`${live.lowStockCount || 0} items`}
          subtext="Stock-out risk"
          change={`${live.outOfStockCount || 0} out of stock`}
          changeType={live.lowStockCount > 0 ? 'negative' : 'neutral'}
          icon={AlertTriangle}
          color="rose"
        />
      </div>

      {/* Live Activity Feed and Today's Pipeline */}
      <div className="grid grid-cols-1 lg:grid-cols-12 gap-6">
        {/* Real-time Ticker / Feed */}
        <div className="lg:col-span-8 bg-white p-6 rounded-2xl border border-gray-100 shadow-sm flex flex-col justify-between">
          <div className="flex items-center justify-between mb-4 pb-3 border-b border-gray-100">
            <div className="flex items-center gap-2">
              <Radio className="w-4 h-4 text-emerald-600 animate-pulse" />
              <h3 className="font-bold text-gray-900 text-sm">Naoja Live Event Stream</h3>
            </div>
            <span className="text-[11px] text-gray-400 font-medium">Auto-updating</span>
          </div>

          <div className="space-y-3 min-h-[260px] max-h-[380px] overflow-y-auto pr-1">
            {(!events || events.length === 0) ? (
              <div className="h-48 flex flex-col items-center justify-center text-center text-gray-400 text-xs">
                <Activity className="w-8 h-8 text-gray-300 mb-2 animate-bounce" />
                <span>Waiting for next customer action... Place an order or trigger payment to see live stream.</span>
              </div>
            ) : (
              events.map((evt, idx) => (
                <div
                  key={evt.id || idx}
                  className="naoja-stream-row p-3.5 rounded-xl border border-gray-100 hover:border-blue-100 bg-white flex items-center justify-between gap-3 text-xs transition-all"
                >
                  <div className="flex items-center gap-3">
                    <div className="w-8 h-8 rounded-lg bg-white shadow-sm flex items-center justify-center shrink-0">
                      {evt.type === 'payment_received' ? (
                        <CheckCircle className="w-4 h-4 text-emerald-600" />
                      ) : evt.type === 'order_created' ? (
                        <ShoppingCart className="w-4 h-4 text-blue-600" />
                      ) : evt.type === 'payment_failed' ? (
                        <AlertTriangle className="w-4 h-4 text-rose-600" />
                      ) : (
                        <Package className="w-4 h-4 text-amber-600" />
                      )}
                    </div>
                    <div>
                      <p className="font-bold text-gray-900">
                        {evt.type === 'payment_received'
                          ? `Payment of KES ${Number(evt.data?.amount || 0).toLocaleString()} Received`
                          : evt.type === 'order_created'
                            ? `Order #${evt.data?.orderRef || 'NEW'} Placed by ${evt.data?.customerName || 'Customer'}`
                            : evt.type === 'payment_failed'
                              ? `M-Pesa STK cancelled or failed: ${evt.data?.resultDesc || 'Transaction Error'}`
                              : `Inventory Alert: ${evt.data?.name || 'Stock Trigger'}`}
                      </p>
                      <p className="text-[11px] text-gray-500">
                        {evt.data?.receipt ? `M-Pesa Receipt: ${evt.data.receipt}` : evt.data?.phone ? `Phone: ${evt.data.phone}` : ''}
                      </p>
                    </div>
                  </div>

                  <div className="flex flex-col items-end gap-1 shrink-0">
                    {getEventBadge(evt.type)}
                    <span className="text-[10px] text-gray-400">
                      {new Date(evt.timestamp).toLocaleTimeString([], { hour: '2-digit', minute: '2-digit', second: '2-digit' })}
                    </span>
                  </div>
                </div>
              ))
            )}
          </div>
        </div>

        {/* Right Info: Storefront Health & Till Details */}
        <div className="lg:col-span-4 space-y-4">
          <div className="bg-white p-5 rounded-2xl border border-gray-100 shadow-sm space-y-4">
            <h4 className="font-bold text-gray-900 text-sm">Storefront Live Diagnostics</h4>

            <div className="space-y-3 text-xs">
              <div className="flex items-center justify-between pb-2 border-b border-gray-50">
                <span className="text-gray-500">M-Pesa Buy Goods Till</span>
                <span className="font-bold text-emerald-700 bg-emerald-50 px-2 py-0.5 rounded">4149288</span>
              </div>
              <div className="flex items-center justify-between pb-2 border-b border-gray-50">
                <span className="text-gray-500">Daraja Gateway Status</span>
                <span className="font-bold text-emerald-600 flex items-center gap-1">
                  <span className="w-2 h-2 rounded-full bg-emerald-500 inline-block"></span> Operational
                </span>
              </div>
              <div className="flex items-center justify-between pb-2 border-b border-gray-50">
                <span className="text-gray-500">Lurambi Store Dispatch</span>
                <span className="font-bold text-gray-800">Open (8:30am - 8:00pm)</span>
              </div>
              <div className="flex items-center justify-between">
                <span className="text-gray-500">Business WhatsApp</span>
                <span className="font-bold text-gray-800">01122079767</span>
              </div>
            </div>
          </div>

          <div className="bg-emerald-50/70 border border-emerald-100 p-5 rounded-2xl space-y-2">
            <div className="flex items-center gap-2 text-emerald-900 font-bold text-xs">
              <ShieldCheck className="w-4 h-4 text-emerald-600" />
              <span>Executive Automation Active</span>
            </div>
            <p className="text-[11px] text-emerald-800 leading-relaxed">
              Order receipts and WhatsApp order updates are automatically dispatched to customers upon M-Pesa PIN confirmation.
            </p>
          </div>
        </div>
      </div>
    </div>
  );
}
