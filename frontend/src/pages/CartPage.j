import { Link } from 'react-router-dom';
import { Trash2, ShoppingBag } from 'lucide-react';
import Layout from '../components/Layout';
import QuantityControl from '../components/QuantityControl';
import FulfillmentModule from '../components/FulfillmentModule';
import { useCart } from '../lib/cart';
import { isFreeDelivery, getDeliveryFee, FREE_DELIVERY_THRESHOLD } from '../lib/fulfillment';

export default function CartPage() {
  const { items, total, updateQuantity, removeItem } = useCart();
  const deliveryFee = getDeliveryFee(total);
  const freeDelivery = isFreeDelivery(total);
  const grandTotal = total + deliveryFee;

  if (items.length === 0) {
    return (
      <Layout>
        <div className="max-w-7xl mx-auto px-4 py-16 text-center">
          <ShoppingBag className="w-16 h-16 text-gray-300 mx-auto mb-4" />
          <h1 className="font-serif text-3xl font-bold text-gray-900 mb-2">Your Cart is Empty</h1>
          <p className="text-gray-500 mb-6">Explore our categories to add electronic and electrical products.</p>
          <Link to="/" className="btn-primary inline-flex">
            Continue Shopping
          </Link>
        </div>
      </Layout>
    );
  }

  return (
    <Layout>
      <div className="max-w-7xl mx-auto px-4 py-6">
        {/* Breadcrumb */}
        <div className="text-xs text-gray-500 pb-4">
          <Link to="/" className="hover:text-brand-500">Home</Link>
          <span className="mx-2">/</span>
          <span className="text-gray-900">Cart</span>
        </div>

        <h1 className="font-serif text-3xl font-bold text-gray-900 mb-6">Shopping Cart</h1>

        <div className="grid lg:grid-cols-5 gap-8 pb-8">
          {/* Cart Items */}
          <div className="lg:col-span-3 bg-white border border-gray-200 rounded-card overflow-hidden shadow-sm h-fit">
            <div className="divide-y divide-gray-100">
              {items.map((item) => (
                <div
                  key={item.product.id}
                  className="grid grid-cols-[80px_1fr_auto] sm:grid-cols-[96px_1fr_auto] gap-4 p-4 sm:p-5 items-center"
                >
                  {/* Image */}
                  <div className="w-20 h-20 sm:w-24 sm:h-24 bg-surface border border-gray-200 rounded-xl overflow-hidden flex items-center justify-center shrink-0">
                    {item.product.image_url ? (
                      <img
                        src={item.product.image_url}
                        alt={item.product.name}
                        className="w-full h-full object-cover"
                      />
                    ) : (
                      <div className="w-full h-full flex items-center justify-center text-gray-300 text-xs">
                        No image
                      </div>
                    )}
                  </div>

                  {/* Info */}
                  <div>
                    <h3 className="font-bold text-gray-900 text-sm sm:text-base line-clamp-2">{item.product.name}</h3>
                    <p className="text-xs text-gray-400 mt-0.5">{item.product.brand}</p>
                    <p className="text-sm font-semibold text-gray-700 mt-2">
                      KSh {item.product.price.toLocaleString()}
                    </p>
                  </div>

                  {/* Actions */}
                  <div className="text-right flex flex-col items-end gap-2.5">
                    <QuantityControl
                      quantity={item.quantity}
                      onIncrease={() => updateQuantity(item.product.id, item.quantity + 1)}
                      onDecrease={() => updateQuantity(item.product.id, item.quantity - 1)}
                      size="sm"
                    />
                    <button
                      onClick={() => removeItem(item.product.id)}
                      className="text-brand-500 text-xs flex items-center gap-1 hover:text-brand-600 font-medium cursor-pointer"
                    >
                      <Trash2 className="w-3.5 h-3.5" />
                      Remove
                    </button>
                  </div>
                </div>
              ))}
            </div>
          </div>

          {/* Order Summary */}
          <div className="lg:col-span-2">
            <div className="bg-white border border-gray-200 rounded-card p-6 sticky top-28 shadow-sm space-y-4">
              <h2 className="font-serif text-xl font-bold text-gray-900 border-b border-gray-100 pb-2">
                Order Summary
              </h2>

              <div className="flex justify-between items-center text-sm">
                <span className="text-gray-500">Subtotal</span>
                <strong className="text-gray-900">KSh {total.toLocaleString()}</strong>
              </div>

              <div className="text-xs space-y-1">
                <div className="flex justify-between items-center text-sm">
                  <span className="text-gray-500">Delivery Fee</span>
                  <strong className="text-gray-900">
                    {freeDelivery ? 'Free' : `KSh ${deliveryFee.toLocaleString()}`}
                  </strong>
                </div>
                {!freeDelivery && (
                  <p className="text-[11px] text-gray-400">
                    Add items worth <strong className="text-gray-600">KSh {(FREE_DELIVERY_THRESHOLD - total).toLocaleString()}</strong> more for free delivery.
                  </p>
                )}
              </div>

              <hr className="border-t border-gray-100" />

              <div className="flex justify-between items-center text-base">
                <strong className="text-gray-900 font-bold">Total</strong>
                <strong className="text-xl font-extrabold text-brand-600">
                  KSh {grandTotal.toLocaleString()}
                </strong>
              </div>

              <Link
                to="/checkout"
                className="btn-primary w-full inline-flex justify-center"
              >
                Proceed to Checkout
              </Link>

              <hr className="border-t border-gray-100" />

              <div>
                <h3 className="font-bold text-xs uppercase text-gray-400 mb-2">Delivery & Hours Information</h3>
                <FulfillmentModule subtotal={total} showPanels={false} />
              </div>
            </div>
          </div>
        </div>
      </div>
    </Layout>
  );
}
