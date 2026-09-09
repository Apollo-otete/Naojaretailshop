import { useState } from 'react';
import { Link, useNavigate } from 'react-router-dom';
import { CheckCircle, ChevronRight, Truck, Store, MapPin, Copy, Check, Smartphone, Loader } from 'lucide-react';
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
  const [formData, setFormData] = useState({ fullName: '', phone: '', area: '', directions: '', fulfillment: 'delivery' });
  const [submitting, setSubmitting] = useState(false);
  const [submitted, setSubmitted] = useState(false);
  const [copiedTill, setCopiedTill] = useState(false);
  const [errors, setErrors] = useState({});
  const [serverError, setServerError] = useState('');

  // STK Push states
  const [stkSent, setStkSent] = useState(false);
  const [stkLoading, setStkLoading] = useState(false);
  const [stkError, setStkError] = useState('');
  const [checkoutRequestId, setCheckoutRequestId] = useState('');
  const [mpesaCode, setMpesaCode] = useState(''); // manual fallback

  const deliveryFee = getDeliveryFee(total);
  const grandTotal = formData.fulfillment === 'delivery' ? total + deliveryFee : total;

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

  // Send STK Push
  async function handleStkPush() {
    setStkLoading(true);
    setStkError('');
    try {
      const res = await fetch('/api/mpesa/stkpush', {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({ phone: formData.phone, amount: grandTotal, orderRef: 'NAOJA-ORDER' }),
      });
      const data = await res.json();
      if (!res.ok) throw new Error(data.message || 'STK Push failed');
      setCheckoutRequestId(data.checkoutRequestId || '');
      setStkSent(true);
    } catch (err) {
      setStkError(err.message);
    } finally {
      setStkLoading(false);
    }
  }

  async function handleConfirm() {
    setSubmitting(true);
    setServerError('');
    try {
      const orderPayload = {
        customer_name: formData.fullName,
        customer_phone: formData.phone,
        total_amount: grandTotal,
        payment_method: 'mpesa',
        mpesa_transaction_id: mpesaCode.trim().toUpperCase(),
        shipping_address: formData.fulfillment === 'delivery' ? `${formData.area}${formData.directions ? ' — ' + formData.directions : ''}` : 'Pickup',
        notes: '',
        items: items.map((i) => ({ product_id: i.product.id, product_name: i.product.name, quantity: i.quantity, unit_price: i.product.price })),
      };
      await api.createCheckout(orderPayload);
      setSubmitted(true);
      clearCart();
    } catch (err) {
      setServerError(err.message || 'Order failed. Please try again.');
    } finally {
      setSubmitting(false);
    }
  }

  if (submitted) {
    return (
      <Layout>
        <div className="max-w-lg mx-auto px-4 py-20 text-center">
          <div className="inline-flex items-center justify-center w-20 h-20 bg-green-100 rounded-full mb-6">
            <CheckCircle className="w-10 h-10 text-green-600" />
          </div>
          <h1 className="font-serif text-3xl font-bold text-gray-900 mb-3">Order Confirmed!</h1>
          <p className="text-gray-500 mb-6">
            Thank you, <strong className="text-gray-900">{formData.fullName}</strong>. We'll process your order shortly.
          </p>
          <Link to="/" className="btn-primary inline-flex">Continue Shopping</Link>
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
                <div className={`w-8 h-8 rounded-full flex items-center justify-center text-xs font-bold transition-colors ${step > i + 1 ? 'bg-brand-500 text-white' : step === i + 1 ? 'bg-brand-500 text-white ring-4 ring-brand-100' : 'bg-gray-100 text-gray-400'}`}>
                  {step > i + 1 ? <Check className="w-4 h-4" /> : i + 1}
                </div>
                <span className={`text-[11px] mt-1 ${step === i + 1 ? 'text-gray-900 font-medium' : 'text-gray-400'}`}>{label}</span>
              </div>
              {i < STEPS.length - 1 && <div className={`h-0.5 w-14 mx-1 mb-5 transition-colors ${step > i + 1 ? 'bg-brand-500' : 'bg-gray-200'}`} />}
            </div>
          ))}
        </div>

        <div className="grid lg:grid-cols-5 gap-8">
          <div className="lg:col-span-3 space-y-4">
            {serverError && <div className="p-4 bg-red-50 text-red-700 rounded-control border border-red-200 text-sm font-semibold">{serverError}</div>}

            {/* STEP 1: Details */}
            {step === 1 && (
              <div className="bg-white border border-gray-200 rounded-card p-5 sm:p-6 shadow-sm space-y-5">
                <div>
                  <h2 className="font-serif text-xl font-bold text-gray-900">Your Details</h2>
                  <p className="text-xs text-gray-400">Please provide your contact info.</p>
                </div>

                <div className="space-y-4">
                  <div>
                    <label className="text-xs font-bold uppercase text-gray-500 block mb-1.5">Full Name</label>
                    <input type="text" placeholder="e.g., Jane Naliaka" value={formData.fullName} onChange={(e) => setFormData({ ...formData, fullName: e.target.value })} className={`w-full h-11 border rounded-control px-3.5 text-sm outline-none focus:border-brand-500 ${errors.fullName ? 'border-red-400' : 'border-gray-200'}`} />
                    {errors.fullName && <p className="text-xs text-red-500 mt-1">{errors.fullName}</p>}
                  </div>
                  <div>
                    <label className="text-xs font-bold uppercase text-gray-500 block mb-1.5">Phone Number (M-Pesa)</label>
                    <input type="tel" placeholder="e.g., 0712345678" value={formData.phone} onChange={(e) => setFormData({ ...formData, phone: e.target.value })} className={`w-full h-11 border rounded-control px-3.5 text-sm outline-none focus:border-brand-500 ${errors.phone ? 'border-red-400' : 'border-gray-200'}`} />
                    {errors.phone && <p className="text-xs text-red-500 mt-1">{errors.phone}</p>}
                  </div>
                </div>

                <hr className="border-gray-100" />

                <div>
                  <h2 className="font-serif text-xl font-bold text-gray-900">Fulfillment</h2>
                  <p className="text-xs text-gray-400">Choose how you want to receive your items.</p>
                </div>

                <div className="grid grid-cols-2 gap-4">
                  {[{ type: 'delivery', Icon: Truck, label: 'Delivery', sub: 'Sun–Fri 8:30am–4:00pm' }, { type: 'pickup', Icon: Store, label: 'Pickup', sub: 'Ready in 1–2 hours' }].map(({ type, Icon, label, sub }) => (
                    <button key={type} type="button" onClick={() => setFormData({ ...formData, fulfillment: type })} className={`h-24 rounded-card border-2 flex flex-col items-center justify-center gap-1.5 transition-colors ${formData.fulfillment === type ? 'border-brand-500 bg-brand-50/50' : 'border-gray-200 hover:border-gray-300'}`}>
                      <Icon className={`w-5 h-5 ${formData.fulfillment === type ? 'text-brand-500' : 'text-gray-400'}`} />
                      <span className="font-bold text-sm text-gray-800">{label}</span>
                      <span className="text-[10px] text-gray-500">{sub}</span>
                    </button>
                  ))}
                </div>

                {formData.fulfillment === 'delivery' && (
                  <div className="space-y-4">
                    <div>
                      <label className="text-xs font-bold uppercase text-gray-500 block mb-1.5">Delivery Area (Kakamega)</label>
                      <input type="text" placeholder="e.g., Lurambi, Amalemba" value={formData.area} onChange={(e) => setFormData({ ...formData, area: e.target.value })} className={`w-full h-11 border rounded-control px-3.5 text-sm outline-none focus:border-brand-500 ${errors.area ? 'border-red-400' : 'border-gray-200'}`} />
                      {errors.area && <p className="text-xs text-red-500 mt-1">{errors.area}</p>}
                    </div>
                    <div>
                      <label className="text-xs font-bold uppercase text-gray-500 block mb-1.5">Directions / Landmark (Optional)</label>
                      <textarea placeholder="e.g., Next to the red kiosk" rows={2} value={formData.directions} onChange={(e) => setFormData({ ...formData, directions: e.target.value })} className="w-full border border-gray-200 rounded-control px-3.5 py-2.5 text-sm outline-none focus:border-brand-500 resize-none" />
                    </div>
                  </div>
                )}

                {formData.fulfillment === 'pickup' && (
                  <div className="bg-surface rounded-card p-4 flex gap-3 border border-gray-200">
                    <MapPin className="w-5 h-5 text-brand-500 mt-0.5 shrink-0" />
                    <div>
                      <p className="text-sm font-bold text-gray-900">{STORE_LOCATION}</p>
                      <p className="text-xs text-gray-400 mt-1">Hours: Sun–Thu 8:30am–8:00pm · Fri 8:30am–3:00pm</p>
                    </div>
                  </div>
                )}

                <button onClick={() => { if (validateStep1()) setStep(2); }} className="btn-primary flex items-center gap-2">
                  Continue to Payment <ChevronRight className="w-4 h-4" />
                </button>
              </div>
            )}

            {/* STEP 2: M-Pesa STK Push */}
            {step === 2 && (
              <div className="bg-white border border-gray-200 rounded-card p-5 sm:p-6 shadow-sm space-y-5">
                <div>
                  <h2 className="font-serif text-xl font-bold text-gray-900">Pay via M-Pesa</h2>
                  <p className="text-xs text-gray-400">We'll send a payment prompt directly to your phone.</p>
                </div>

                {/* Amount callout */}
                <div className="bg-brand-50 border border-brand-200 rounded-card p-4 flex justify-between items-center">
                  <div>
                    <p className="text-[10px] text-brand-700 font-bold uppercase">Amount Due</p>
                    <p className="font-extrabold text-2xl text-brand-700">KSh {grandTotal.toLocaleString()}</p>
                  </div>
                  <div className="text-right">
                    <p className="text-[10px] text-gray-400 uppercase font-bold">Buy Goods Till</p>
                    <p className="font-mono font-extrabold text-2xl text-gray-900">{TILL_NUMBER}</p>
                    <button onClick={copyTillNumber} className="mt-1 flex items-center gap-1 text-xs text-brand-500 ml-auto">
                      {copiedTill ? <><Check className="w-3.5 h-3.5 text-green-600" /> Copied!</> : <><Copy className="w-3.5 h-3.5" /> Copy Till</>}
                    </button>
                  </div>
                </div>

                {/* STK Push section */}
                {!stkSent ? (
                  <div className="space-y-4">
                    <div className="bg-surface rounded-card p-4 border border-gray-200 text-xs text-gray-600 space-y-2">
                      <p className="font-bold text-gray-800">How it works:</p>
                      <p>1. Click <strong>Send Payment Request</strong> below.</p>
                      <p>2. An M-Pesa prompt will appear on <strong>{formData.phone}</strong>.</p>
                      <p>3. Enter your M-Pesa PIN to pay KSh {grandTotal.toLocaleString()}.</p>
                      <p>4. Your order will be automatically confirmed!</p>
                    </div>

                    {stkError && <div className="p-3 bg-red-50 border border-red-200 text-red-700 text-xs rounded-xl">{stkError}</div>}

                    <button onClick={handleStkPush} disabled={stkLoading} className="w-full h-12 bg-green-600 hover:bg-green-700 disabled:opacity-70 text-white rounded-xl font-bold text-sm flex items-center justify-center gap-2 transition-colors">
                      {stkLoading ? <><Loader className="w-4 h-4 animate-spin" /> Sending Request…</> : <><Smartphone className="w-4 h-4" /> Send M-Pesa Request to {formData.phone}</>}
                    </button>
                    <p className="text-center text-xs text-gray-400">Or pay manually using the Till number above, then enter the code below.</p>
                  </div>
                ) : (
                  // Waiting screen after STK sent
                  <div className="space-y-4">
                    <div className="bg-green-50 border border-green-200 rounded-xl p-5 text-center space-y-3">
                      <Loader className="w-8 h-8 text-green-600 animate-spin mx-auto" />
                      <p className="font-bold text-green-800">Payment request sent!</p>
                      <p className="text-xs text-green-700">Check <strong>{formData.phone}</strong> for the M-Pesa PIN prompt and enter your PIN to complete.</p>
                    </div>
                    <button onClick={() => { setStkSent(false); setStkError(''); }} className="text-xs text-gray-500 hover:text-gray-700 underline w-full text-center">
                      Didn't receive it? Resend
                    </button>
                  </div>
                )}

                {/* Manual code fallback — always visible */}
                <div>
                  <label className="text-xs font-bold uppercase text-gray-500 block mb-1.5">
                    M-Pesa Transaction Code <span className="text-gray-400 normal-case font-normal">(if you already paid manually)</span>
                  </label>
                  <input type="text" placeholder="e.g., RG7X2YZABC" value={mpesaCode} onChange={(e) => setMpesaCode(e.target.value.toUpperCase())} className="w-full h-11 border border-gray-200 rounded-control px-3.5 text-sm font-mono outline-none focus:border-brand-500 uppercase tracking-wider" />
                </div>

                <div className="flex gap-3 pt-2">
                  <button onClick={() => setStep(3)} className="btn-primary flex items-center gap-2">
                    Continue to Review <ChevronRight className="w-4 h-4" />
                  </button>
                  <button onClick={() => setStep(1)} className="btn-secondary">Back</button>
                </div>
              </div>
            )}

            {/* STEP 3: Review & Confirm */}
            {step === 3 && (
              <div className="bg-white border border-gray-200 rounded-card p-5 sm:p-6 shadow-sm space-y-5">
                <div>
                  <h2 className="font-serif text-xl font-bold text-gray-900">Review & Confirm</h2>
                  <p className="text-xs text-gray-400">Double-check before placing your order.</p>
                </div>

                <div className="grid grid-cols-1 sm:grid-cols-2 gap-4">
                  <div className="bg-surface rounded-card p-4 border border-gray-200">
                    <p className="text-[10px] text-gray-400 font-bold uppercase mb-2">Customer & Delivery</p>
                    <p className="text-sm font-bold text-gray-800">{formData.fullName}</p>
                    <p className="text-xs text-gray-500">{formData.phone}</p>
                    <p className="text-xs text-gray-600 mt-2">
                      {formData.fulfillment === 'delivery' ? `Delivery to ${formData.area}` : 'Pickup at store'}
                    </p>
                  </div>
                  <div className="bg-surface rounded-card p-4 border border-gray-200">
                    <p className="text-[10px] text-gray-400 font-bold uppercase mb-2">Payment</p>
                    <p className="text-xs text-gray-600">Method: M-Pesa ({stkSent ? 'STK Push' : 'Manual'})</p>
                    {mpesaCode && <p className="text-xs font-mono font-bold text-brand-600 mt-1">Code: {mpesaCode}</p>}
                    {!mpesaCode && stkSent && <p className="text-xs text-green-600 font-semibold mt-1">✓ Payment prompt sent</p>}
                  </div>
                </div>

                <div className="border border-gray-100 rounded-card overflow-hidden">
                  <div className="bg-surface px-4 py-2 border-b border-gray-100 text-[10px] uppercase font-bold text-gray-500">Items</div>
                  {items.map((item) => (
                    <div key={item.product.id} className="flex justify-between items-center px-4 py-3 text-sm border-b border-gray-50 last:border-0">
                      <span className="text-gray-700">{item.product.name} × {item.quantity}</span>
                      <strong className="text-gray-900">KSh {(item.product.price * item.quantity).toLocaleString()}</strong>
                    </div>
                  ))}
                </div>

                <div className="flex gap-3 pt-2">
                  <button onClick={handleConfirm} disabled={submitting} className="btn-primary flex items-center justify-center gap-2 min-w-44">
                    {submitting ? 'Placing Order...' : 'Place Order'} {!submitting && <CheckCircle className="w-4 h-4" />}
                  </button>
                  <button onClick={() => setStep(2)} className="btn-secondary">Back</button>
                </div>
              </div>
            )}
          </div>

          {/* Order Summary sidebar */}
          <div className="lg:col-span-2">
            <div className="bg-white border border-gray-200 rounded-card p-5 sticky top-28 shadow-sm space-y-4">
              <h3 className="font-serif text-lg font-bold text-gray-900 border-b border-gray-100 pb-2">Order Overview</h3>
              <div className="space-y-3 max-h-48 overflow-y-auto">
                {items.map((item) => (
                  <div key={item.product.id} className="flex justify-between text-xs items-center">
                    <span className="text-gray-500 truncate max-w-[160px]">{item.product.name}</span>
                    <span className="text-gray-400">× {item.quantity}</span>
                    <strong className="text-gray-800">KSh {(item.product.price * item.quantity).toLocaleString()}</strong>
                  </div>
                ))}
              </div>
              <hr className="border-gray-100" />
              <div className="space-y-2 text-xs">
                <div className="flex justify-between text-gray-500"><span>Subtotal</span><strong className="text-gray-800">KSh {total.toLocaleString()}</strong></div>
                <div className="flex justify-between text-gray-500"><span>Delivery</span><strong className="text-gray-800">{formData.fulfillment === 'delivery' ? (isFreeDelivery(total) ? 'Free' : `KSh ${deliveryFee.toLocaleString()}`) : 'KSh 0'}</strong></div>
              </div>
              <hr className="border-gray-100" />
              <div className="flex justify-between items-center text-sm">
                <strong className="text-gray-900">Total</strong>
                <strong className="text-lg font-extrabold text-brand-600">KSh {grandTotal.toLocaleString()}</strong>
              </div>
            </div>
          </div>
        </div>
      </div>
    </Layout>
  );
}
