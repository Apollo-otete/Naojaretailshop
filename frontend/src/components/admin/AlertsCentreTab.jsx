import React, { useState } from 'react';
import { AlertTriangle, AlertCircle, Info, Check, CheckCheck, Clock, ShieldAlert } from 'lucide-react';
import { analyticsApi } from '../../lib/analyticsApi';

export default function AlertsCentreTab({ alertsData, onAcknowledge, onRefresh }) {
  const [filter, setFilter] = useState('all');
  const [actingId, setActingId] = useState(null);

  const alerts = alertsData || [];

  const filteredAlerts = alerts.filter((a) => {
    if (filter === 'all') return true;
    return a.severity === filter;
  });

  const handleAck = async (id) => {
    setActingId(id);
    try {
      await analyticsApi.acknowledgeAlert(id);
      if (onAcknowledge) onAcknowledge(id);
      if (onRefresh) onRefresh();
    } catch (err) {
      alert('Error acknowledging alert: ' + err.message);
    } finally {
      setActingId(null);
    }
  };

  const getSeverityBadge = (sev) => {
    switch (sev) {
      case 'critical':
        return (
          <span className="inline-flex items-center gap-1 px-2.5 py-1 rounded-full text-xs font-bold bg-rose-100 text-rose-800 border border-rose-200">
            <ShieldAlert className="w-3.5 h-3.5 text-rose-600" /> Critical Severity
          </span>
        );
      case 'warning':
        return (
          <span className="inline-flex items-center gap-1 px-2.5 py-1 rounded-full text-xs font-bold bg-amber-100 text-amber-800 border border-amber-200">
            <AlertTriangle className="w-3.5 h-3.5 text-amber-600" /> Warning
          </span>
        );
      default:
        return (
          <span className="inline-flex items-center gap-1 px-2.5 py-1 rounded-full text-xs font-bold bg-blue-100 text-blue-800 border border-blue-200">
            <Info className="w-3.5 h-3.5 text-blue-600" /> Information
          </span>
        );
    }
  };

  return (
    <div className="space-y-6">
      {/* Header & Filter Controls */}
      <div className="bg-white p-5 rounded-2xl border border-gray-100 shadow-sm flex flex-col sm:flex-row sm:items-center justify-between gap-4">
        <div>
          <h2 className="text-lg font-bold text-gray-900">Alerts & Operational Watchdog</h2>
          <p className="text-xs text-gray-500">
            Automated alerts triggered by low stock, transaction anomalies, and stuck orders
          </p>
        </div>

        {/* Severity Filter */}
        <div className="inline-flex p-1 bg-gray-100 rounded-xl text-xs font-bold">
          {[
            { id: 'all', label: `All Alerts (${alerts.length})` },
            { id: 'critical', label: 'Critical' },
            { id: 'warning', label: 'Warnings' },
            { id: 'info', label: 'Info' }
          ].map((tab) => (
            <button
              key={tab.id}
              onClick={() => setFilter(tab.id)}
              className={`px-3 py-1.5 rounded-lg transition-all ${
                filter === tab.id
                  ? 'bg-gray-900 text-white shadow-sm'
                  : 'text-gray-600 hover:text-gray-900'
              }`}
            >
              {tab.label}
            </button>
          ))}
        </div>
      </div>

      {/* Alerts List */}
      <div className="space-y-3">
        {filteredAlerts.length === 0 ? (
          <div className="bg-white p-12 rounded-2xl border border-gray-100 shadow-sm text-center">
            <CheckCheck className="w-12 h-12 text-emerald-500 mx-auto mb-3" />
            <h3 className="text-base font-bold text-gray-900">All Clear! No Active Alerts</h3>
            <p className="text-xs text-gray-500 mt-1 max-w-sm mx-auto">
              Warehouse inventory levels, payment gateways, and fulfillment pipelines are operating smoothly.
            </p>
          </div>
        ) : (
          filteredAlerts.map((alert) => (
            <div
              key={alert.id}
              className={`p-5 rounded-2xl border transition-all flex flex-col sm:flex-row sm:items-center justify-between gap-4 ${
                alert.severity === 'critical'
                  ? 'bg-rose-50/40 border-rose-200'
                  : alert.severity === 'warning'
                  ? 'bg-amber-50/40 border-amber-200'
                  : 'bg-white border-gray-100 shadow-sm'
              }`}
            >
              <div className="flex items-start gap-4">
                <div
                  className={`w-10 h-10 rounded-xl flex items-center justify-center shrink-0 mt-0.5 ${
                    alert.severity === 'critical'
                      ? 'bg-rose-100 text-rose-700'
                      : alert.severity === 'warning'
                      ? 'bg-amber-100 text-amber-700'
                      : 'bg-blue-100 text-blue-700'
                  }`}
                >
                  {alert.severity === 'critical' ? (
                    <ShieldAlert className="w-5 h-5" />
                  ) : alert.severity === 'warning' ? (
                    <AlertTriangle className="w-5 h-5" />
                  ) : (
                    <Info className="w-5 h-5" />
                  )}
                </div>

                <div className="space-y-1">
                  <div className="flex flex-wrap items-center gap-2">
                    <h4 className="text-sm font-bold text-gray-900">{alert.title}</h4>
                    {getSeverityBadge(alert.severity)}
                  </div>
                  <p className="text-xs text-gray-600 leading-relaxed">{alert.message}</p>
                  <div className="flex items-center gap-2 text-[11px] text-gray-400 pt-1">
                    <Clock className="w-3.5 h-3.5" />
                    <span>{new Date(alert.created_at).toLocaleString()}</span>
                  </div>
                </div>
              </div>

              <div className="shrink-0 self-end sm:self-center">
                <button
                  disabled={actingId === alert.id}
                  onClick={() => handleAck(alert.id)}
                  className="inline-flex items-center gap-1.5 px-3.5 py-1.5 bg-white hover:bg-gray-100 border border-gray-300 text-gray-700 text-xs font-bold rounded-xl shadow-sm transition-all"
                >
                  <Check className="w-4 h-4 text-emerald-600" />
                  <span>{actingId === alert.id ? 'Resolving...' : 'Mark Resolved'}</span>
                </button>
              </div>
            </div>
          ))
        )}
      </div>
    </div>
  );
}
