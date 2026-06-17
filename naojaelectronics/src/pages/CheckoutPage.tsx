import { useState } from 'react';
import { Link } from 'react-router-dom';
import { CheckCircle, ChevronRight, Truck, Store, MapPin, Clock, Copy, Check } from 'lucide-react';
import Layout from '../components/Layout';
import { useCart } from '../lib/cart';
import { getDeliveryFee, isFreeDelivery, STORE_LOCATION, formatDeliveryMessage, formatPickupMessage } from '../lib/fulfillment';

const TILL_NUMBER = '4149288';
const TILL_NAME = 'Naoja Electrical & Electronics';

const STEPS = ['Details', 'Payment', 'Confirm'];

export default function CheckoutPage() {
  const { items, total, clearCart } = useCart();
  const [step, setStep] = useState(1);
  const [formData, setFormData] = useState({
    fullName: '',
    phone: '',
    area: '',
    directions: '',
    fulfillment: 'delivery' as 'delivery' | 'pickup',
  });
  const [mpesaRef, setMpesaRef] = useState('');
  const [submitting, setSubmitting] = useState(false);
  const [submitted, setSubmitted] = useState(false);
  const [copiedTill, setCopiedTill] = useState(false);
  const [errors, setErrors] = useState<Record<string, string>>({});

  const deliveryFee = getDeliveryFee(total);
  const grandTotal = total + deliveryFee;

  function copyTillNumber() {
    navigator.clipboard.writeText(TILL_NUMBER).then(() => {
      setCopiedTill(true);
      setTimeout(() => setCopiedTill(false), 2000);
    });
  }

  function validateStep1() {
    const e: Record<string, string> = {};
    if (!formData.fullName.trim()) e.fullName = 'Name is required';
    if (!formData.phone.trim()) e.phone = 'Phone number is required';
    if (formData.fulfillment === 'delivery' && !formData.area.trim()) e.area = 'Area is required for delivery';
    setErrors(e);
    return Object.keys(e).length === 0;
  }

  function validateStep2() {
    const e: Record<string, string> = {};
    if (!mpesaRef.trim()) e.mpesaRef = 'M-Pesa transaction code is required';
    setErrors(e);
    return Object.keys(e).length === 0;
  }

  async function handleConfirm() {
    setSubmitting(true);
    try {
      await new Promise((r) => setTimeout(r, 1200));
      setSubmitted(true);
      clearCart();
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
          <h1 className="font-serif text-3xl text-gray-900 mb-2">Order Confirmed!</h1>
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
            <p className="font-mono font-bold text-gray-900">{mpesaRef.toUpperCase()}</p>
          </div>
          <Link to="/" className="btn-primary inline-flex">
            Continue Shopping
          </Link>
        </div>
      </Layout>
    );
  }

  if (items.length === 0) {
    return (
      <Layout>
        <div className="max-w-7xl mx-auto px-4 py-12 text-center">
          <h1 className="font-serif text-3xl text-gray-900 mb-2">Your cart is empty</h1>
          <Link to="/" className="btn-primary inline-flex">
            Continue Shopping
          </Link>
        </div>
      </Layout>
    );
  }

  return (
    <Layout>
      <div className="max-w-7xl mx-auto px-4 pb-12">
        {/* Breadcrumb */}
        <div className="text-xs text-gray-500 py-4">
          <Link to="/">Home</Link>
          <span className="mx-2">/</span>
          <Link to="/cart">Cart</Link>
          <span className="mx-2">/</span>
          <span className="text-gray-900">Checkout</span>
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
                <span className={`text-xs mt-1 ${step === i + 1 ? 'text-gray-900 font-medium' : 'text-gray-400'}`}>
                  {label}
                </span>
              </div>
              {i < STEPS.length - 1 && (
                <div className={`h-0.5 w-14 mx-1 mb-5 transition-colors ${step > i + 1 ? 'bg-brand-500' : 'bg-gray-200'}`} />
              )}
            </div>
          ))}
        </div>

        <div className="grid lg:grid-cols-5 gap-6">
          {/* Left: Steps */}
          <div className="lg:col-span-3 space-y-4">

            {/* STEP 1: Details */}
            {step === 1 && (
              <div className="bg-white border border-gray-200 rounded-card p-5">
                <h2 className="font-serif text-xl text-gray-900 mb-4">Your Details</h2>

                <div className="space-y-3 mb-5">
                  <div>
                    <label className="text-xs text-gray-500 block mb-1.5">Full name</label>
                    <input
                      type="text"
                      placeholder="e.g., Jane Naliaka"
                      value={formData.fullName}
                      onChange={(e) => setFormData({ ...formData, fullName: e.target.value })}
                      className={`w-full h-11 border rounded-control px-3 text-sm outline-none focus:border-brand-500 ${errors.fullName ? 'border-red-400' : 'border-gray-200'}`}
                    />
                    {errors.fullName && <p className="text-xs text-red-500 mt-1">{errors.fullName}</p>}
                  </div>

                  <div>
                    <label className="text-xs text-gray-500 block mb-1.5">Phone number (M-Pesa)</label>
                    <input
                      type="tel"
                      placeholder="07XX XXX XXX"
                      value={formData.phone}
                      onChange={(e) => setFormData({ ...formData, phone: e.target.value })}
                      className={`w-full h-11 border rounded-control px-3 text-sm outline-none focus:border-brand-500 ${errors.phone ? 'border-red-400' : 'border-gray-200'}`}
                    />
                    {errors.phone && <p className="text-xs text-red-500 mt-1">{errors.phone}</p>}
                  </div>
                </div>

                <hr className="border-gray-200 mb-5" />

                <h2 className="font-serif text-xl text-gray-900 mb-3">Fulfillment</h2>

                <div className="grid grid-cols-2 gap-3 mb-4">
                  <button
                    type="button"
                    className={`h-24 rounded-card border-2 flex flex-col items-center justify-center gap-1.5 transition-colors ${
                      formData.fulfillment === 'delivery'
                        ? 'border-brand-500 bg-brand-50'
                        : 'border-gray-200 hover:border-gray-300'
                    }`}
                    onClick={() => setFormData({ ...formData, fulfillment: 'delivery' })}
                  >
                    <Truck className={`w-5 h-5 ${formData.fulfillment === 'delivery' ? 'text-brand-500' : 'text-gray-400'}`} />
                    <span className="font-bold text-sm">Delivery</span>
                    <span className="text-xs text-gray-500">Sun–Fri 8am–4pm</span>
                  </button>
                  <button
                    type="button"
                    className={`h-24 rounded-card border-2 flex flex-col items-center justify-center gap-1.5 transition-colors ${
                      formData.fulfillment === 'pickup'
                        ? 'border-brand-500 bg-brand-50'
                        : 'border-gray-200 hover:border-gray-300'
                    }`}
                    onClick={() => setFormData({ ...formData, fulfillment: 'pickup' })}
                  >
                    <Store className={`w-5 h-5 ${formData.fulfillment === 'pickup' ? 'text-brand-500' : 'text-gray-400'}`} />
                    <span className="font-bold text-sm">Pickup</span>
                    <span className="text-xs text-gray-500">Ready in 1–2 hrs</span>
                  </button>
                </div>

                {formData.fulfillment === 'delivery' && (
                  <div className="space-y-3 mb-4">
                    <div>
                      <label className="text-xs text-gray-500 block mb-1.5">Area / Estate</label>
                      <input
                        type="text"
                        placeholder="e.g., Lurambi"
                        value={formData.area}
                        onChange={(e) => setFormData({ ...formData, area: e.target.value })}
                        className={`w-full h-11 border rounded-control px-3 text-sm outline-none focus:border-brand-500 ${errors.area ? 'border-red-400' : 'border-gray-200'}`}
                      />
                      {errors.area && <p className="text-xs text-red-500 mt-1">{errors.area}</p>}
                    </div>
                    <div>
                      <label className="text-xs text-gray-500 block mb-1.5">Directions / Landmark <span className="text-gray-400">(optional)</span></label>
                      <textarea
                        placeholder="e.g., near the blue gate..."
                        value={formData.directions}
                        onChange={(e) => setFormData({ ...formData, directions: e.target.value })}
                        className="w-full h-20 border border-gray-200 rounded-control px-3 py-2.5 text-sm outline-none focus:border-brand-500 resize-none"
                      />
                    </div>
                  </div>
                )}

                {formData.fulfillment === 'pickup' && (
                  <div className="bg-surface rounded-card p-3.5 mb-4 flex gap-2.5">
                    <MapPin className="w-4 h-4 text-brand-500 mt-0.5 shrink-0" />
                    <div>
                      <p className="text-sm font-semibold text-gray-900">{STORE_LOCATION}</p>
                      <p className="text-xs text-gray-500 mt-1">{formatPickupMessage()}</p>
                      <p className="text-xs text-gray-400 mt-1">Sun–Thu 8am–8pm · Fri 8am–4pm · Closed Sat</p>
                    </div>
                  </div>
                )}

                <div className="flex gap-3">
                  <button
                    onClick={() => { if (validateStep1()) setStep(2); }}
                    className="btn-primary flex items-center gap-2"
                  >
                    Continue <ChevronRight className="w-4 h-4" />
                  </button>
                  <Link to="/cart" className="btn-secondary">
                    Back
                  </Link>
                </div>
              </div>
            )}

            {/* STEP 2: M-Pesa Payment Instructions */}
            {step === 2 && (
              <div className="bg-white border border-gray-200 rounded-card p-5">
                <h2 className="font-serif text-xl text-gray-900 mb-1">Pay via M-Pesa</h2>
                <p className="text-sm text-gray-500 mb-5">Follow the steps below to complete payment, then enter your M-Pesa code.</p>

                {/* Amount to pay */}
                <div className="bg-brand-50 border border-brand-200 rounded-card p-4 mb-5 flex items-center justify-between">
                  <div>
                    <p className="text-xs text-brand-700 font-semibold uppercase tracking-wide">Amount to Pay</p>
                    <p className="font-extrabold text-3xl text-brand-700 mt-0.5">KSh {grandTotal.toLocaleString()}</p>
                  </div>
                  <div className="text-right">
                    <p className="text-xs text-gray-500">Buy Goods Till</p>
                    <p className="font-mono font-extrabold text-2xl text-gray-900">{TILL_NUMBER}</p>
                    <button
                      onClick={copyTillNumber}
                      className="mt-1 flex items-center gap-1 text-xs text-brand-500 hover:underline ml-auto"
                    >
                      {copiedTill ? <><Check className="w-3 h-3" /> Copied!</> : <><Copy className="w-3 h-3" /> Copy number</>}
                    </button>
                  </div>
                </div>

                {/* Step-by-step instructions */}
                <div className="bg-surface rounded-card p-4 mb-5">
                  <p className="text-xs font-semibold text-gray-700 uppercase tracking-wide mb-3">How to pay</p>
                  <ol className="space-y-3">
                    {[
                      { step: '1', text: 'Open M-Pesa on your phone' },
                      { step: '2', text: 'Select Lipa na M-Pesa' },
                      { step: '3', text: 'Select Buy Goods and Services' },
                      { step: '4', text: <>Enter Till Number: <strong className="font-mono text-gray-900 text-sm">{TILL_NUMBER}</strong></> },
                      { step: '5', text: <>Enter amount: <strong className="text-gray-900">KSh {grandTotal.toLocaleString()}</strong></> },
                      { step: '6', text: 'Enter your M-Pesa PIN and confirm' },
                      { step: '7', text: <>Copy the <strong className="text-gray-900">transaction code</strong> from the SMS sent to your phone and paste it below</> },
                    ].map(({ step: s, text }) => (
                      <li key={s} className="flex items-start gap-3">
                        <span className="w-6 h-6 rounded-full bg-brand-500 text-white text-xs font-bold flex items-center justify-center shrink-0">
                          {s}
                        </span>
                        <span className="text-sm text-gray-600 pt-0.5">{text}</span>
                      </li>
                    ))}
                  </ol>
                </div>

                {/* Till details card */}
                <div className="border border-gray-200 rounded-card p-4 mb-5">
                  <p className="text-xs text-gray-500 mb-2 uppercase tracking-wide font-semibold">Payment Details</p>
                  <div className="grid grid-cols-2 gap-y-2 text-sm">
                    <span className="text-gray-500">Till Number</span>
                    <span className="font-mono font-bold text-gray-900">{TILL_NUMBER}</span>
                    <span className="text-gray-500">Till Name</span>
                    <span className="font-medium text-gray-900">{TILL_NAME}</span>
                    <span className="text-gray-500">Amount</span>
                    <span className="font-bold text-gray-900">KSh {grandTotal.toLocaleString()}</span>
                  </div>
                </div>

                {/* M-Pesa code input */}
                <div className="mb-5">
                  <label className="text-sm text-gray-700 font-semibold block mb-1.5">
                    M-Pesa Transaction Code
                  </label>
                  <p className="text-xs text-gray-500 mb-2">
                    This is the code in the confirmation SMS from M-Pesa (e.g., <span className="font-mono">RG7X2YZABC</span>).
                  </p>
                  <input
                    type="text"
                    placeholder="e.g., RG7X2YZABC"
                    value={mpesaRef}
                    onChange={(e) => setMpesaRef(e.target.value.toUpperCase())}
                    className={`w-full h-12 border rounded-control px-3 text-sm font-mono outline-none focus:border-brand-500 uppercase tracking-wider ${errors.mpesaRef ? 'border-red-400' : 'border-gray-200'}`}
                  />
                  {errors.mpesaRef && <p className="text-xs text-red-500 mt-1">{errors.mpesaRef}</p>}
                </div>

                <div className="flex gap-3">
                  <button
                    onClick={() => { if (validateStep2()) setStep(3); }}
                    className="btn-primary flex items-center gap-2"
                  >
                    Verify &amp; Continue <ChevronRight className="w-4 h-4" />
                  </button>
                  <button onClick={() => setStep(1)} className="btn-secondary">
                    Back
                  </button>
                </div>
              </div>
            )}

            {/* STEP 3: Review & Confirm */}
            {step === 3 && (
              <div className="bg-white border border-gray-200 rounded-card p-5">
                <h2 className="font-serif text-xl text-gray-900 mb-4">Review Order</h2>

                {/* Customer */}
                <div className="bg-surface rounded-card p-4 mb-3">
                  <p className="text-xs text-gray-500 uppercase tracking-wide font-semibold mb-2">Customer</p>
                  <p className="text-sm font-semibold text-gray-900">{formData.fullName}</p>
                  <p className="text-sm text-gray-500">{formData.phone}</p>
                </div>

                {/* Fulfillment */}
                <div className="bg-surface rounded-card p-4 mb-3">
                  <p className="text-xs text-gray-500 uppercase tracking-wide font-semibold mb-2">Fulfillment</p>
                  {formData.fulfillment === 'delivery' ? (
                    <>
                      <div className="flex items-center gap-2 mb-1">
                        <Truck className="w-4 h-4 text-brand-500" />
                        <span className="text-sm font-semibold text-gray-900">Delivery</span>
                      </div>
                      <p className="text-sm text-gray-500">{formData.area}</p>
                      {formData.directions && <p className="text-sm text-gray-500">{formData.directions}</p>}
                      <p className="text-xs text-gray-400 mt-1 flex items-center gap-1">
                        <Clock className="w-3 h-3" /> {formatDeliveryMessage()}
                      </p>
                    </>
                  ) : (
                    <>
                      <div className="flex items-center gap-2 mb-1">
                        <Store className="w-4 h-4 text-brand-500" />
                        <span className="text-sm font-semibold text-gray-900">Pickup</span>
                      </div>
                      <p className="text-sm text-gray-500">{STORE_LOCATION}</p>
                      <p className="text-xs text-gray-400 mt-1 flex items-center gap-1">
                        <Clock className="w-3 h-3" /> {formatPickupMessage()}
                      </p>
                    </>
                  )}
                </div>

                {/* Payment */}
                <div className="bg-surface rounded-card p-4 mb-3">
                  <p className="text-xs text-gray-500 uppercase tracking-wide font-semibold mb-2">Payment</p>
                  <div className="flex items-center justify-between">
                    <div>
                      <p className="text-sm font-semibold text-gray-900">M-Pesa Buy Goods</p>
                      <p className="text-xs text-gray-500">Till {TILL_NUMBER} · {TILL_NAME}</p>
                    </div>
                    <div className="text-right">
                      <p className="text-xs text-gray-500">Ref</p>
                      <p className="font-mono font-bold text-gray-900">{mpesaRef}</p>
                    </div>
                  </div>
                </div>

                {/* Items */}
                <div className="border border-gray-200 rounded-card overflow-hidden mb-4">
                  {items.map((item, i) => (
                    <div key={item.product.id} className={`flex items-center gap-3 p-3 ${i < items.length - 1 ? 'border-b border-gray-100' : ''}`}>
                      {item.product.image_url && (
                        <img src={item.product.image_url} alt={item.product.name} className="w-10 h-10 rounded-lg object-cover" />
                      )}
                      <div className="flex-1 min-w-0">
                        <p className="text-sm font-medium text-gray-900 truncate">{item.product.name}</p>
                        <p className="text-xs text-gray-500">Qty: {item.quantity}</p>
                      </div>
                      <p className="text-sm font-bold text-gray-900 shrink-0">
                        KSh {(item.product.price * item.quantity).toLocaleString()}
                      </p>
                    </div>
                  ))}
                </div>

                <div className="flex gap-3">
                  <button
                    onClick={handleConfirm}
                    disabled={submitting}
                    className="btn-primary flex items-center gap-2"
                  >
                    {submitting ? 'Confirming...' : 'Confirm Order'}
                    {!submitting && <CheckCircle className="w-4 h-4" />}
                  </button>
                  <button onClick={() => setStep(2)} className="btn-secondary">
                    Back
                  </button>
                </div>
              </div>
            )}
          </div>

          {/* Right: Order Summary */}
          <div className="lg:col-span-2">
            <div className="bg-white border border-gray-200 rounded-card p-4 sticky top-40">
              <h2 className="font-serif text-xl text-gray-900 mb-3">Order Summary</h2>

              <div className="space-y-1.5 mb-4">
                {items.map((item) => (
                  <div key={item.product.id} className="flex justify-between text-sm">
                    <span className="text-gray-600 truncate pr-2">{item.product.name} ×{item.quantity}</span>
                    <span className="text-gray-900 font-medium shrink-0">KSh {(item.product.price * item.quantity).toLocaleString()}</span>
                  </div>
                ))}
              </div>

              <hr className="border-gray-200 mb-3" />

              <div className="space-y-1.5 text-sm mb-4">
                <div className="flex justify-between text-gray-500">
                  <span>Subtotal</span>
                  <span className="text-gray-900">KSh {total.toLocaleString()}</span>
                </div>
                <div className="flex justify-between text-gray-500">
                  <span>Delivery</span>
                  <span className={isFreeDelivery(total) ? 'text-green-600 font-medium' : 'text-gray-900'}>
                    {isFreeDelivery(total) ? 'Free' : `KSh ${deliveryFee}`}
                  </span>
                </div>
              </div>

              <hr className="border-gray-200 mb-3" />

              <div className="flex justify-between font-bold text-base mb-5">
                <span>Total</span>
                <span>KSh {grandTotal.toLocaleString()}</span>
              </div>

              {/* M-Pesa Till reminder */}
              <div className="bg-brand-50 border border-brand-200 rounded-card p-3">
                <p className="text-xs text-brand-700 font-semibold mb-1">Pay via M-Pesa Buy Goods</p>
                <p className="text-xs text-brand-600">
                  Till Number: <strong className="font-mono text-lg">{TILL_NUMBER}</strong>
                </p>
                <p className="text-xs text-brand-600">{TILL_NAME}</p>
              </div>
            </div>
          </div>
        </div>
      </div>
    </Layout>
  );
}
