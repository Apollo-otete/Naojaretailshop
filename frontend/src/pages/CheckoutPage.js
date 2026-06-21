import { useState } from 'react';
import { Link, useNavigate } from 'react-router-dom';
import { CheckCircle, ChevronRight, Truck, Store, MapPin, Clock, Copy, Check } from 'lucide-react';
import Layout from '../components/Layout';
import { useCart } from '../lib/cart';
import { api } from '../lib/api';
import { getDeliveryFee, isFreeDelivery, STORE_LOCATION, formatDeliveryMessage, formatPickupMessage } from '../lib/fulfillment';

const TILL_NUMBER = '4149288';
const TILL_NAME = 'Naoja Ventures';

const STEPS = ['Details', 'Payment', 'Confirm'];

export default function CheckoutPage() {
  const { items, total, clearCart } = useCart();
  const navigate = useNavigate();
  const [step, setStep] = useState(1);
  const [formData, setFormData] = useState({
    fullName: '',
    phone: '',
    area: '',
    directions: '',
    fulfillment: 'delivery',
  });
  const [mpesaRef, setMpesaRef] = useState('');
  const [submitting, setSubmitting] = useState(false);
  const [submitted, setSubmitted] = useState(false);
  const [copiedTill, setCopiedTill] = useState(false);
  const [errors, setErrors] = useState({});
  const [serverError, setServerError] = useState('');

  const deliveryFee = getDeliveryFee(total);
  const grandTotal = total + deliveryFee;

  function copyTillNumber() {
    navigator.clipboard.writeText(TILL_NUMBER).then(() => {
      setCopiedTill(true);
      setTimeout(() => setCopiedTill(false), 2000);
    });
  }

  function validateStep1() {
    const e = {};
    if (!formData.fullName.trim()) e.fullName = 'Name is required';
    if (!formData.phone.trim()) e.phone = 'Phone number is required';
    if (formData.fulfillment === 'delivery' && !formData.area.trim()) e.area = 'Area is required for delivery';
    setErrors(e);
    return Object.keys(e).length === 0;
  }

  function validateStep2() {
    const e = {};
    if (!mpesaRef.trim()) e.mpesaRef = 'M-Pesa transaction code is required';
    // Basic validation for transaction code (usually 10 alphanumeric chars)
    if (mpesaRef.trim().length < 8) e.mpesaRef = 'Invalid M-Pesa transaction code format';
    setErrors(e);
    return Object.keys(e).length === 0;
  }

  async function handleConfirm() {
    setSubmitting(true);
    setServerError('');
    try {
      const orderPayload = {
        customerName: formData.fullName,
        customerPhone: formData.phone,
        fulfillmentType: formData.fulfillment,
        deliveryArea: formData.fulfillment === 'delivery' ? formData.area : null,
        deliveryDirections: formData.fulfillment === 'delivery' ? formData.directions : null,
        mpesaCode: mpesaRef.trim().toUpperCase(),
        subtotal: total,
        deliveryFee: formData.fulfillment === 'delivery' ? deliveryFee : 0,
        total: formData.fulfillment === 'delivery' ? grandTotal : total,
        items: items.map((item) => ({
          productId: item.product.id,
          productName: item.product.name,
          quantity: item.quantity,
          price: item.product.price,
        })),
      };

      await api.createCheckout(orderPayload);
      setSubmitted(true);
      clearCart();
    } catch (err) {
      console.error('Checkout failed:', err);
      setServerError(err.message || 'An error occurred during order confirmation. Please try again.');
    } finally {
      setSubmitting(false);
    }
  }

  if (submitted) {
    return (
      <Layout>
        <div className="max-w-lg mx-auto px-4 py-16 text-center">
          <div className="inline-flex items-center justify-center w-20 h-20 bg-green-100 rounded-full mb-5">
            <CheckCircle className="w-10 h-10 text-green-600" />
          </div>
          <h1 className="font-serif text-3xl font-bold text-gray-900 mb-2">Order Confirmed!</h1>
          <p className="text-gray-500 mb-2">
            Thank you, <strong className="text-gray-900">{formData.fullName}</strong>.
          </p>
          <p className="text-gray-500 mb-6">
            {formData.fulfillment === 'delivery'
              ? `Your order will be delivered to ${formData.area}. ${formatDeliveryMessage()}`
              : `Your order will be ready for pickup at ${STORE_LOCATION} in 1–2 hours.`}
          </p>
          <div className="bg-surface border border-gray-200 rounded-card p-4 text-left mb-6">
            <p className="text-xs text-gray-500 mb-1">M-Pesa Reference</p>
            <p className="font-mono font-bold text-gray-900 text-base">{mpesaRef.toUpperCase()}</p>
          </div>
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
          <Link to="/cart" className="hover:text-brand-500">Cart</Link>
          <span className="mx-2">/</span>
          <span className="text-gray-900 font-medium">Checkout</span>
        </div>

        {/* Step indicator */}
        <div className="flex items-center gap-0 mb-8 max-w-sm">
          {STEPS.map((label, i) => (
            <div key={label} className="flex items-center">
              <div className="flex flex-col items-center">
                <div
                  className={`w-8 h-8 rounded-full flex items-center justify-center text-xs font-bold transition-colors ${
                    step > i + 1
                      ? 'bg-brand-500 text-white'
                      : step === i + 1
                      ? 'bg-brand-500 text-white ring-4 ring-brand-100'
                      : 'bg-gray-100 text-gray-400'
                  }`}
                >
                  {step > i + 1 ? <Check className="w-4 h-4" /> : i + 1}
                </div>
                <span className={`text-[11px] mt-1 ${step === i + 1 ? 'text-gray-900 font-medium' : 'text-gray-400'}`}>
                  {label}
                </span>
              </div>
              {i < STEPS.length - 1 && (
                <div className={`h-0.5 w-14 mx-1 mb-5 transition-colors ${step > i + 1 ? 'bg-brand-500' : 'bg-gray-200'}`} />
              )}
            </div>
          ))}
        </div>

        <div className="grid lg:grid-cols-5 gap-8">
          {/* Left Panel: Steps */}
          <div className="lg:col-span-3 space-y-4">
            {serverError && (
              <div className="p-4 bg-red-50 text-red-700 rounded-control border border-red-200 text-sm font-semibold">
                {serverError}
              </div>
            )}

            {/* STEP 1: Details */}
            {step === 1 && (
              <div className="bg-white border border-gray-200 rounded-card p-5 sm:p-6 shadow-sm space-y-5">
                <div>
                  <h2 className="font-serif text-xl font-bold text-gray-900">Your Details</h2>
                  <p className="text-xs text-gray-400">Please provide contact info so we can coordinate your order.</p>
                </div>

                <div className="space-y-4">
                  <div>
                    <label className="text-xs font-bold uppercase text-gray-500 block mb-1.5">Full Name</label>
                    <input
                      type="text"
                      placeholder="e.g., Jane Naliaka"
                      value={formData.fullName}
                      onChange={(e) => setFormData({ ...formData, fullName: e.target.value })}
                      className={`w-full h-11 border rounded-control px-3.5 text-sm outline-none focus:border-brand-500 ${errors.fullName ? 'border-red-400 bg-red-50/10' : 'border-gray-200'}`}
                    />
                    {errors.fullName && <p className="text-xs text-red-500 mt-1">{errors.fullName}</p>}
                  </div>

                  <div>
                    <label className="text-xs font-bold uppercase text-gray-500 block mb-1.5">Phone Number (M-Pesa)</label>
                    <input
                      type="tel"
                      placeholder="e.g., 0712345678"
                      value={formData.phone}
                      onChange={(e) => setFormData({ ...formData, phone: e.target.value })}
                      className={`w-full h-11 border rounded-control px-3.5 text-sm outline-none focus:border-brand-500 ${errors.phone ? 'border-red-400 bg-red-50/10' : 'border-gray-200'}`}
                    />
                    {errors.phone && <p className="text-xs text-red-500 mt-1">{errors.phone}</p>}
                  </div>
                </div>

                <hr className="border-gray-100" />

                <div>
                  <h2 className="font-serif text-xl font-bold text-gray-900">Fulfillment</h2>
                  <p className="text-xs text-gray-400">Choose how you want to receive your items.</p>
                </div>

                <div className="grid grid-cols-2 gap-4">
                  <button
                    type="button"
                    className={`h-24 rounded-card border-2 flex flex-col items-center justify-center gap-1.5 transition-colors cursor-pointer ${
                      formData.fulfillment === 'delivery'
                        ? 'border-brand-500 bg-brand-50/50'
                        : 'border-gray-200 hover:border-gray-300'
                    }`}
                    onClick={() => setFormData({ ...formData, fulfillment: 'delivery' })}
                  >
                    <Truck className={`w-5 h-5 ${formData.fulfillment === 'delivery' ? 'text-brand-500' : 'text-gray-400'}`} />
                    <span className="font-bold text-sm text-gray-800">Delivery</span>
                    <span className="text-[10px] text-gray-500">Sun–Fri 8:30am–4:00pm</span>
                  </button>
                  <button
                    type="button"
                    className={`h-24 rounded-card border-2 flex flex-col items-center justify-center gap-1.5 transition-colors cursor-pointer ${
                      formData.fulfillment === 'pickup'
                        ? 'border-brand-500 bg-brand-50/50'
                        : 'border-gray-200 hover:border-gray-300'
                    }`}
                    onClick={() => setFormData({ ...formData, fulfillment: 'pickup' })}
                  >
                    <Store className={`w-5 h-5 ${formData.fulfillment === 'pickup' ? 'text-brand-500' : 'text-gray-400'}`} />
                    <span className="font-bold text-sm text-gray-800">Pickup</span>
                    <span className="text-[10px] text-gray-500">Ready in 1–2 hours</span>
                  </button>
                </div>

                {formData.fulfillment === 'delivery' && (
                  <div className="space-y-4">
                    <div>
                      <label className="text-xs font-bold uppercase text-gray-500 block mb-1.5">Delivery Area / Estate (Kakamega)</label>
                      <input
                        type="text"
                        placeholder="e.g., Lurambi, Kefinco, Amalemba"
                        value={formData.area}
                        onChange={(e) => setFormData({ ...formData, area: e.target.value })}
                        className={`w-full h-11 border rounded-control px-3.5 text-sm outline-none focus:border-brand-500 ${errors.area ? 'border-red-400 bg-red-50/10' : 'border-gray-200'}`}
                      />
                      {errors.area && <p className="text-xs text-red-500 mt-1">{errors.area}</p>}
                    </div>
                    <div>
                      <label className="text-xs font-bold uppercase text-gray-500 block mb-1.5">Directions / Landmark (Optional)</label>
                      <textarea
                        placeholder="e.g., Next to the red kiosk, third gate on the left"
                        rows={2}
                        value={formData.directions}
                        onChange={(e) => setFormData({ ...formData, directions: e.target.value })}
                        className="w-full border border-gray-200 rounded-control px-3.5 py-2.5 text-sm outline-none focus:border-brand-500 resize-none"
                      />
                    </div>
                  </div>
                )}

                {formData.fulfillment === 'pickup' && (
                  <div className="bg-surface rounded-card p-4 flex gap-3 border border-gray-200">
                    <MapPin className="w-5 h-5 text-brand-500 mt-0.5 shrink-0" />
                    <div>
                      <p className="text-sm font-bold text-gray-900">{STORE_LOCATION}</p>
                      <p className="text-xs text-gray-500 mt-1">{formatPickupMessage()}</p>
                      <p className="text-xs text-gray-400 mt-1.5">Hours: Sun-Thu 8:30am-8:00pm · Fri 8:30am-3:00pm</p>
                    </div>
                  </div>
                )}

                <div className="flex gap-3 pt-2">
                  <button
                    onClick={() => { if (validateStep1()) setStep(2); }}
                    className="btn-primary flex items-center gap-2 cursor-pointer"
                  >
                    Continue to Payment <ChevronRight className="w-4 h-4" />
                  </button>
                  <Link to="/cart" className="btn-secondary">
                    Back to Cart
                  </Link>
                </div>
              </div>
            )}

            {/* STEP 2: M-Pesa Payment */}
            {step === 2 && (
              <div className="bg-white border border-gray-200 rounded-card p-5 sm:p-6 shadow-sm space-y-5">
                <div>
                  <h2 className="font-serif text-xl font-bold text-gray-900">Pay via M-Pesa</h2>
                  <p className="text-xs text-gray-400">Complete payment via Till, then input the confirmation transaction code.</p>
                </div>

                {/* Till Callout */}
                <div className="bg-brand-50 border border-brand-200 rounded-card p-4 flex flex-col sm:flex-row justify-between items-start sm:items-center gap-4">
                  <div>
                    <p className="text-[10px] text-brand-700 font-bold uppercase tracking-wider">Amount Due</p>
                    <p className="font-extrabold text-2xl text-brand-700 mt-0.5">KSh {grandTotal.toLocaleString()}</p>
                  </div>
                  <div className="sm:text-right">
                    <p className="text-[10px] text-gray-400 uppercase font-bold">Buy Goods Till</p>
                    <p className="font-mono font-extrabold text-2xl text-gray-900 tracking-tight">{TILL_NUMBER}</p>
                    <button
                      onClick={copyTillNumber}
                      className="mt-1 flex items-center gap-1 text-xs text-brand-500 hover:text-brand-600 font-medium ml-auto cursor-pointer"
                    >
                      {copiedTill ? <><Check className="w-3.5 h-3.5 text-green-600" /> Copied!</> : <><Copy className="w-3.5 h-3.5" /> Copy Till</>}
                    </button>
                  </div>
                </div>

                {/* Instructions */}
                <div className="bg-surface rounded-card p-4 border border-gray-200">
                  <p className="text-xs font-bold text-gray-700 uppercase tracking-wider mb-3">Lipa na M-Pesa Steps</p>
                  <ol className="space-y-3">
                    {[
                      { s: '1', t: 'Open the M-Pesa menu or App on your phone.' },
                      { s: '2', t: 'Select Lipa na M-Pesa, then Buy Goods and Services.' },
                      { s: '3', t: <>Enter Till Number: <strong className="font-mono text-gray-900 text-sm">{TILL_NUMBER}</strong></> },
                      { s: '4', t: <>Enter Amount: <strong className="text-gray-900">KSh {grandTotal.toLocaleString()}</strong></> },
                      { s: '5', t: 'Enter your M-Pesa PIN and complete transaction.' },
                      { s: '6', t: 'Paste the transaction code from the confirmation SMS in the field below.' },
                    ].map(({ s, t }) => (
                      <li key={s} className="flex items-start gap-3">
                        <span className="w-5 h-5 rounded-full bg-brand-500 text-white text-[10px] font-bold flex items-center justify-center shrink-0 mt-0.5">
                          {s}
                        </span>
                        <span className="text-xs text-gray-600 leading-relaxed">{t}</span>
                      </li>
                    ))}
                  </ol>
                </div>

                {/* Code Field */}
                <div>
                  <label className="text-xs font-bold uppercase text-gray-500 block mb-1.5">
                    M-Pesa Transaction Code
                  </label>
                  <input
                    type="text"
                    placeholder="e.g., RG7X2YZABC"
                    value={mpesaRef}
                    onChange={(e) => setMpesaRef(e.target.value.toUpperCase())}
                    className={`w-full h-11 border rounded-control px-3.5 text-sm font-mono outline-none focus:border-brand-500 uppercase tracking-wider ${errors.mpesaRef ? 'border-red-400 bg-red-50/10' : 'border-gray-200'}`}
                  />
                  {errors.mpesaRef && <p className="text-xs text-red-500 mt-1">{errors.mpesaRef}</p>}
                </div>

                <div className="flex gap-3 pt-2">
                  <button
                    onClick={() => { if (validateStep2()) setStep(3); }}
                    className="btn-primary flex items-center gap-2 cursor-pointer"
                  >
                    Verify &amp; Continue <ChevronRight className="w-4 h-4" />
                  </button>
                  <button onClick={() => setStep(1)} className="btn-secondary">
                    Back
                  </button>
                </div>
              </div>
            )}

            {/* STEP 3: Confirm */}
            {step === 3 && (
              <div className="bg-white border border-gray-200 rounded-card p-5 sm:p-6 shadow-sm space-y-5">
                <div>
                  <h2 className="font-serif text-xl font-bold text-gray-900">Review &amp; Confirm</h2>
                  <p className="text-xs text-gray-400">Double check order details before final submission.</p>
                </div>

                <div className="grid grid-cols-1 sm:grid-cols-2 gap-4">
                  {/* Delivery Info */}
                  <div className="bg-surface rounded-card p-4 border border-gray-200">
                    <p className="text-[10px] text-gray-400 font-bold uppercase tracking-wider mb-2">Customer &amp; Delivery</p>
                    <p className="text-sm font-bold text-gray-800">{formData.fullName}</p>
                    <p className="text-xs text-gray-500">{formData.phone}</p>
                    {formData.fulfillment === 'delivery' ? (
                      <div className="mt-2 text-xs text-gray-600">
                        <span className="font-semibold">Fulfillment:</span> Delivery to {formData.area}
                        {formData.directions && <p className="italic text-gray-400 mt-1">"{formData.directions}"</p>}
                      </div>
                    ) : (
                      <div className="mt-2 text-xs text-gray-600">
                        <span className="font-semibold">Fulfillment:</span> Pickup at Lurambi Opp. Bamboo
                      </div>
                    )}
                  </div>

                  {/* Payment Info */}
                  <div className="bg-surface rounded-card p-4 border border-gray-200">
                    <p className="text-[10px] text-gray-400 font-bold uppercase tracking-wider mb-2">M-Pesa Verification</p>
                    <p className="text-xs text-gray-600"><span className="font-semibold">Till Number:</span> {TILL_NUMBER}</p>
                    <p className="text-xs text-gray-600"><span className="font-semibold">Till Name:</span> {TILL_NAME}</p>
                    <div className="mt-2.5 pt-2 border-t border-gray-100 flex justify-between items-center">
                      <span className="text-xs text-gray-500">M-Pesa Reference Code:</span>
                      <strong className="font-mono text-sm text-brand-600">{mpesaRef}</strong>
                    </div>
                  </div>
                </div>

                {/* Items Summary list */}
                <div className="border border-gray-150 rounded-card overflow-hidden">
                  <div className="bg-surface px-4 py-2 border-b border-gray-150 text-[10px] uppercase font-bold text-gray-500">
                    Items in Order
                  </div>
                  <div className="divide-y divide-gray-100 bg-white">
                    {items.map((item) => (
                      <div key={item.product.id} className="flex justify-between items-center p-3 text-sm">
                        <span className="text-gray-700 truncate max-w-xs sm:max-w-md">
                          {item.product.name} × {item.quantity}
                        </span>
                        <strong className="text-gray-900 shrink-0 font-semibold">
                          KSh {(item.product.price * item.quantity).toLocaleString()}
                        </strong>
                      </div>
                    ))}
                  </div>
                </div>

                <div className="flex gap-3 pt-2">
                  <button
                    onClick={handleConfirm}
                    disabled={submitting}
                    className="btn-primary flex items-center justify-center gap-2 cursor-pointer min-w-44"
                  >
                    {submitting ? 'Placing Order...' : 'Place Order'}
                    {!submitting && <CheckCircle className="w-4 h-4" />}
                  </button>
                  <button onClick={() => setStep(2)} className="btn-secondary">
                    Back
                  </button>
                </div>
              </div>
            )}
          </div>

          {/* Right Panel: Order Pricing */}
          <div className="lg:col-span-2">
            <div className="bg-white border border-gray-200 rounded-card p-5 sticky top-28 shadow-sm space-y-4">
              <h3 className="font-serif text-lg font-bold text-gray-900 border-b border-gray-100 pb-2">
                Order Overview
              </h3>

              <div className="space-y-3.5 max-h-48 overflow-y-auto pr-1">
                {items.map((item) => (
                  <div key={item.product.id} className="flex justify-between text-xs items-center">
                    <span className="text-gray-500 truncate max-w-[160px]">{item.product.name}</span>
                    <span className="text-gray-400">× {item.quantity}</span>
                    <strong className="text-gray-800 font-semibold">KSh {(item.product.price * item.quantity).toLocaleString()}</strong>
                  </div>
                ))}
              </div>

              <hr className="border-t border-gray-100" />

              <div className="space-y-2 text-xs">
                <div className="flex justify-between text-gray-500">
                  <span>Subtotal</span>
                  <strong className="text-gray-800">KSh {total.toLocaleString()}</strong>
                </div>
                <div className="flex justify-between text-gray-500">
                  <span>Delivery Fee</span>
                  <strong className="text-gray-800">
                    {formData.fulfillment === 'delivery'
                      ? isFreeDelivery(total) ? 'Free' : `KSh ${deliveryFee.toLocaleString()}`
                      : 'KSh 0'
                    }
                  </strong>
                </div>
              </div>

              <hr className="border-t border-gray-100" />

              <div className="flex justify-between items-center text-sm">
                <strong className="text-gray-900 font-bold">Total</strong>
                <strong className="text-lg font-extrabold text-brand-600">
                  KSh {
                    formData.fulfillment === 'delivery'
                      ? grandTotal.toLocaleString()
                      : total.toLocaleString()
                  }
                </strong>
              </div>
            </div>
          </div>
        </div>
      </div>
    </Layout>
  );
}
