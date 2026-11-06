import { useState, useEffect } from 'react';
import { Link } from 'react-router-dom';
import {
  formatDeliveryMessage,
  formatPickupMessage,
  isFreeDelivery,
  STORE_LOCATION,
  FREE_DELIVERY_THRESHOLD,
} from '../lib/fulfillment';

export default function FulfillmentModule({
  subtotal,
  showPanels = true,
}) {
  const [tab, setTab] = useState('delivery');
  const [deliveryMsg, setDeliveryMsg] = useState('');
  const [pickupMsg, setPickupMsg] = useState('');

  useEffect(() => {
    setDeliveryMsg(formatDeliveryMessage());
    setPickupMsg(formatPickupMessage());

    const interval = setInterval(() => {
      setDeliveryMsg(formatDeliveryMessage());
      setPickupMsg(formatPickupMessage());
    }, 60000);

    return () => clearInterval(interval);
  }, []);

  const freeDelivery = isFreeDelivery(subtotal);

  return (
    <div className="border border-gray-200 rounded-card p-4 bg-white">
      {showPanels && (
        <div className="flex gap-2 bg-surface p-1.5 rounded-xl border border-gray-200 mb-3">
          <button
            className={`flex-1 py-2.5 px-3 rounded-control font-bold text-xs transition-colors cursor-pointer ${
              tab === 'delivery'
                ? 'bg-white border border-gray-200 text-gray-900'
                : 'text-gray-500'
            }`}
            onClick={() => setTab('delivery')}
          >
            Delivery
          </button>
          <button
            className={`flex-1 py-2.5 px-3 rounded-control font-bold text-xs transition-colors cursor-pointer ${
              tab === 'pickup'
                ? 'bg-white border border-gray-200 text-gray-900'
                : 'text-gray-500'
            }`}
            onClick={() => setTab('pickup')}
          >
            Pickup
          </button>
        </div>
      )}

      {/* Delivery Panel */}
      {(!showPanels || tab === 'delivery') && (
        <div>
          <p className="text-xs text-gray-500">
            {freeDelivery ? (
              <>
                Delivery: <strong className="text-gray-900">Free</strong> (orders &ge; KSh {FREE_DELIVERY_THRESHOLD})
              </>
            ) : (
              <>
                Delivery: <span className="text-gray-500 font-medium">Free from KSh {FREE_DELIVERY_THRESHOLD} (Current Fee: KSh 150)</span>
              </>
            )}
          </p>

          {showPanels && (
            <>
              <p className="text-xs text-gray-500 mt-2">{deliveryMsg}</p>
              <p className="text-xs text-gray-500 mt-2">
                Delivery is available <strong>Sunday-Friday</strong>. No deliveries are made on <strong>Saturday</strong>.
              </p>
              <Link
                to="/account?tab=policy"
                className="inline-flex text-brand-500 text-xs font-medium mt-2 hover:underline"
              >
                Delivery & Pickup details
              </Link>
            </>
          )}
        </div>
      )}

      {/* Pickup Panel */}
      {showPanels && tab === 'pickup' && (
        <div>
          <p className="text-xs text-gray-500">
            Pickup point: <strong className="text-gray-900">{STORE_LOCATION}</strong>
          </p>
          <p className="text-xs text-gray-500 mt-2">{pickupMsg}</p>
          <p className="text-xs text-gray-500 mt-2">
            Online orders are accepted <strong>24/7</strong>. Pickup is available <strong>daily</strong>.
          </p>
          <Link
            to="/account?tab=location"
            className="inline-flex text-brand-500 text-xs font-medium mt-2 hover:underline"
          >
            View store location
          </Link>
        </div>
      )}
    </div>
  );
}
