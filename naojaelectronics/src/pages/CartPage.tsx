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
        <div className="max-w-7xl mx-auto px-4 py-12 text-center">
          <ShoppingBag className="w-16 h-16 text-gray-300 mx-auto mb-4" />
          <h1 className="font-serif text-3xl text-gray-900 mb-2">Your cart is empty</h1>
          <p className="text-gray-500 mb-6">Add some products to get started</p>
          <Link to="/" className="btn-primary inline-flex">
            Continue Shopping
          </Link>
        </div>
      </Layout>
    );
  }

  return (
    <Layout>
      <div className="max-w-7xl mx-auto px-4">
        {/* Breadcrumb */}
        <div className="text-xs text-gray-500 py-4">
          <Link to="/">Home</Link>
          <span className="mx-2">/</span>
          <span className="text-gray-900">Cart</span>
        </div>

        <h1 className="font-serif text-3xl text-gray-900 mb-6">Shopping Cart</h1>

        <div className="grid lg:grid-cols-5 gap-6 pb-8">
          {/* Cart Items */}
          <div className="lg:col-span-3 bg-white border border-gray-200 rounded-card overflow-hidden">
            {items.map((item, index) => (
              <div
                key={item.product.id}
                className={`grid grid-cols-[96px_1fr_auto] gap-4 p-4 ${
                  index < items.length - 1 ? 'border-b border-gray-200' : ''
                }`}
              >
                {/* Image */}
                <div className="w-24 h-24 bg-surface border border-gray-200 rounded-xl overflow-hidden">
                  {item.product.image_url ? (
                    <img
                      src={item.product.image_url}
                      alt={item.product.name}
                      className="w-full h-full object-cover"
                    />
                  ) : (
                    <div className="w-full h-full flex items-center justify-center text-gray-300">
                      No img
                    </div>
                  )}
                </div>

                {/* Info */}
                <div>
                  <h3 className="font-extrabold text-gray-900">{item.product.name}</h3>
                  <p className="text-xs text-gray-500 mt-1">{item.product.brand}</p>
                  <p className="text-sm text-gray-500 mt-2">
                    KSh {item.product.price.toLocaleString()}
                  </p>
                </div>

                {/* Actions */}
                <div className="text-right">
                  <QuantityControl
                    quantity={item.quantity}
                    onIncrease={() => updateQuantity(item.product.id, item.quantity + 1)}
                    onDecrease={() => updateQuantity(item.product.id, item.quantity - 1)}
                    size="sm"
                  />
                  <button
                    onClick={() => removeItem(item.product.id)}
                    className="mt-2.5 text-brand-500 text-xs flex items-center gap-1 hover:underline ml-auto"
                  >
                    <Trash2 className="w-3 h-3" />
                    Remove
                  </button>
                </div>
              </div>
            ))}
          </div>

          {/* Summary */}
          <div className="lg:col-span-2">
            <div className="bg-white border border-gray-200 rounded-card p-4 sticky top-40">
              <h2 className="font-serif text-xl text-gray-900 mb-3">Order Summary</h2>

              <div className="text-sm text-gray-500 mb-2">
                Subtotal: <strong className="text-gray-900">KSh {total.toLocaleString()}</strong>
              </div>

              <div className="text-xs text-gray-500 mb-1">
                {freeDelivery ? (
                  <>
                    Delivery: <strong className="text-gray-900">Free</strong> (orders &ge; KSh {FREE_DELIVERY_THRESHOLD})
                  </>
                ) : (
                  <>
                    Delivery: <span className="text-gray-400">Free from KSh {FREE_DELIVERY_THRESHOLD}</span>
                    <strong className="block text-gray-900">KSh {deliveryFee}</strong>
                  </>
                )}
              </div>

              <hr className="border-t border-gray-200 my-4" />

              <div className="text-sm">
                <strong>Total:</strong> KSh {grandTotal.toLocaleString()}
              </div>

              <Link
                to="/checkout"
                className="btn-primary w-full mt-4 inline-flex justify-center"
              >
                Proceed to Checkout
              </Link>

              <hr className="border-t border-gray-200 my-4" />

              <h3 className="font-semibold text-sm mb-2.5">Fulfillment</h3>
              <FulfillmentModule subtotal={total} showPanels={true} compact />
            </div>
          </div>
        </div>
      </div>
    </Layout>
  );
}
