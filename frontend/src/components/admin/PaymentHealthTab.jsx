import React from 'react';
import { CreditCard, CheckCircle2, AlertOctagon, Clock, Smartphone, ShieldCheck } from 'lucide-react';
import KpiCard from './KpiCard';

export default function PaymentHealthTab({ paymentData, range, setRange }) {
  const payment = paymentData || {};
  const failureReasons = payment.failureReasons || [];

  return (
    <div className="space-y-6">
      {/* Top Header & Range Slicer */}
      <div className="flex flex-col sm:flex-row sm:items-center justify-between gap-4 bg-white p-5 rounded-2xl border border-gray-100 shadow-sm">
        <div>
          <h2 className="text-lg font-bold text-gray-900">M-Pesa & Payment Health Diagnostics</h2>
          <p className="text-xs text-gray-500">
            Safaricom Daraja STK Push reliability, conversion percentages, and drop-off analysis
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
          title="Payment Success Rate"
          value={`${payment.successRate || 95}%`}
          subtext="Cleared STK Pushes"
          change={payment.successRate >= 90 ? 'Optimal' : 'Investigate'}
          changeType={payment.successRate >= 90 ? 'positive' : 'negative'}
          icon={CheckCircle2}
          color="brand"
        />

        <KpiCard
          title="Total Transactions"
          value={payment.totalTransactions || 0}
          subtext="STK prompts sent"
          change={`${payment.completed || 0} paid`}
          changeType="positive"
          icon={CreditCard}
          color="blue"
        />

        <KpiCard
          title="Failed / Cancelled"
          value={payment.failed || 0}
          subtext="Prompt drop-offs"
          change={`${payment.failed || 0} orders`}
          changeType={payment.failed > 5 ? 'negative' : 'neutral'}
          icon={AlertOctagon}
          color="rose"
        />

        <KpiCard
          title="Avg Time-to-Pay"
          value={`${payment.avgTimeToPaySeconds || 14}s`}
          subtext="PIN entry speed"
          change="Safaricom Network"
          changeType="neutral"
          icon={Clock}
          color="amber"
        />
      </div>

      {/* Detailed Failure Breakdown & Daraja Configuration */}
      <div className="grid grid-cols-1 lg:grid-cols-12 gap-6">
        {/* Failure Reasons Breakdown */}
        <div className="lg:col-span-7 bg-white p-6 rounded-2xl border border-gray-100 shadow-sm space-y-4">
          <div>
            <h3 className="font-bold text-gray-900 text-base">M-Pesa Failure Diagnostics (ResultCode Analysis)</h3>
            <p className="text-xs text-gray-400">
              Why transactions did not complete on Safaricom customer SIM cards
            </p>
          </div>

          <div className="space-y-3">
            {failureReasons.map((item, idx) => (
              <div key={idx} className="p-3.5 rounded-xl border border-gray-100 bg-gray-50/50 space-y-2">
                <div className="flex items-center justify-between text-xs">
                  <div className="flex items-center gap-2">
                    <span className="font-mono text-[11px] bg-gray-200 px-1.5 py-0.5 rounded text-gray-800">
                      {item.code}
                    </span>
                    <span className="font-bold text-gray-900">{item.reason}</span>
                  </div>
                  <span className="font-bold text-rose-600">{item.count} occurrences</span>
                </div>
                <div className="w-full bg-gray-200 h-2 rounded-full overflow-hidden">
                  <div
                    className="bg-rose-500 h-full rounded-full"
                    style={{
                      width: `${payment.failed > 0 ? (item.count / payment.failed) * 100 : 0}%`
                    }}
                  ></div>
                </div>
              </div>
            ))}
          </div>
        </div>

        {/* M-Pesa Till 4149288 Details Card */}
        <div className="lg:col-span-5 bg-gradient-to-br from-emerald-900 to-gray-900 text-white p-6 rounded-2xl shadow-sm space-y-4 flex flex-col justify-between">
          <div>
            <div className="flex items-center gap-2 mb-2">
              <ShieldCheck className="w-5 h-5 text-emerald-400" />
              <h3 className="font-bold text-base">Storefront Daraja Till</h3>
            </div>
            <p className="text-xs text-emerald-200/80 leading-relaxed">
              Your store is configured with Safaricom Daraja Lipa Na M-Pesa Buy Goods Till.
            </p>
          </div>

          <div className="space-y-3 text-xs pt-3 border-t border-emerald-800/60">
            <div className="flex justify-between">
              <span className="text-gray-300">Store Till Number:</span>
              <span className="font-bold text-emerald-300">4149288</span>
            </div>
            <div className="flex justify-between">
              <span className="text-gray-300">Environment:</span>
              <span className="font-bold text-emerald-300">Safaricom Daraja API</span>
            </div>
            <div className="flex justify-between">
              <span className="text-gray-300">Instant Receipts:</span>
              <span className="font-bold text-emerald-300">Automated SMS & Email</span>
            </div>
            <div className="flex justify-between">
              <span className="text-gray-300">Helpline / Support Phone:</span>
              <span className="font-bold text-emerald-300">0112079767</span>
            </div>
          </div>

          <div className="p-3 rounded-xl bg-white/10 text-[11px] text-gray-200">
            Tip: Over 85% of dropped transactions occur when buyers mistakenly lock their phone screen before entering their PIN.
          </div>
        </div>
      </div>
    </div>
  );
}
