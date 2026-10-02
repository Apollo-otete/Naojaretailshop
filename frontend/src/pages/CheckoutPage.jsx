import React, { useState, useEffect, useRef } from 'react';
import { Link, useNavigate } from 'react-router-dom';
import { CheckCircle, ChevronRight, Truck, Store, MapPin, Copy, Check, Smartphone, Loader, AlertCircle } from 'lucide-react';
import Layout from '../components/Layout';
import { useCart } from '../lib/cart';
import { api } from '../lib/api';
import { getDeliveryFee, isFreeDelivery, STORE_LOCATION } from '../lib/fulfillment';

const TILL_NUMBER = '4149288';
const TILL_NAME = 'Naoja Ventures';
const STEPS = ['Details', 'Payment', 'Confirm'];

export default function CheckoutPage() {
  const { items, total, clearCart } = useCart();
  const navigate = useNavigate();
  const [step, setStep] = useState(1);
  const [formData, setFormData] = useState({ fullName: '', phone: '', email: '', area: '', directions: '', fulfillment: 'delivery' });
  const [submitting, setSubmitting] = useState(false);
  const [submitted, setSubmitted] = useState(false);
  const [confirmedOrder, setConfirmedOrder] = useState(null);
  const [createdOrder, setCreatedOrder] = useState(null);
  const [copiedTill, setCopiedTill] = useState(false);
  const [copiedRef, setCopiedRef] = useState(false);
  const [errors, setErrors] = useState({});
  const [serverError, setServerError] = useState('');

  // STK Push states
  const [stkSent, setStkSent] = useState(false);
  const [stkLoading, setStkLoading] = useState(false);
  const [stkError, setStkError] = useState('');
  const [checkoutRequestId, setCheckoutRequestId] = useState('');
  const [mpesaCode, setMpesaCode] = useState(''); // manual fallback
  const [paymentSuccess, setPaymentSuccess] = useState(false);

  const pollIntervalRef = useRef(null);

  const deliveryFee = getDeliveryFee(total);
  const grandTotal = formData.fulfillment === 'delivery' ? total + deliveryFee : total;

  function copyTillNumber() {
    navigator.clipboard.writeText(TILL_NUMBER).then(() => {
      setCopiedTill(true);
      setTimeout(() => setCopiedTill(false), 2000);
    });
  }

  function copyOrderRef(ref) {
    if (!ref) return;
    navigator.clipboard.writeText(ref).then(() => {
      setCopiedRef(true);
      setTimeout(() => setCopiedRef(false), 2000);
    });
  }

  function validateStep1() {
    const e = {};
    if (!formData.fullName.trim()) e.fullName = 'Name is required';
    if (!formData.phone.trim()) e.phone = 'Phone number is required';
    if (formData.fulfillment === 'delivery' && !formData.area.trim()) e.area = 'Delivery area is required';
    setErrors(e);
    return Object.keys(e).length === 0;
  }

  // Cleanup polling timer on unmount
  useEffect(() => {
    return () => {
      if (pollIntervalRef.current) clearInterval(pollIntervalRef.current);
    };
  }, []);

  // Helper to ensure order is registered in database
  async function ensureOrderCreated(transactionCode = '') {
    if (createdOrder) return createdOrder;

    const orderPayload = {
      customer_name: formData.fullName,
      customer_phone: formData.phone,
      customer_email: formData.email || '',
      total_amount: grandTotal,
      payment_method: 'mpesa',
      payment_status: 'pending',
      mpesa_transaction_id: transactionCode || '',
      shipping_address: formData.fulfillment === 'delivery' ? `${formData.area}${formData.directions ? ' — ' + formData.directions : ''}` : 'Store Pickup (Kakamega, Lurambi)',
      notes: '',
      items: items.map((i) => ({
        product_id: i.product.id,
        product_name: i.product.name,
        quantity: i.quantity,
        unit_price: i.product.price,
        total_price: i.product.price * i.quantity,
      })),
    };

    const res = await api.createCheckout(orderPayload);
    const order = res.order || res;
    setCreatedOrder(order);
    return order;
  }

  // Start polling backend for order payment status
  function startPaymentPolling(orderRef, reqId) {
    if (pollIntervalRef.current) clearInterval(pollIntervalRef.current);

    let attempts = 0;
    const maxAttempts = 25; // poll for 75 seconds (3s * 25)

    pollIntervalRef.current = setInterval(async () => {
      attempts++;
      try {
        // 1. Check order status directly
        if (orderRef) {
          const statusRes = await api.mpesa.getOrderStatus(orderRef);
          if (statusRes && statusRes.payment_status === 'paid') {
            clearInterval(pollIntervalRef.current);
            setPaymentSuccess(true);
            setConfirmedOrder({ ...createdOrder, ...statusRes });
            clearCart();
            setSubmitted(true);
            return;
          }
        }

        // 2. Query Safaricom if checkoutRequestId exists
        if (reqId && attempts % 2 === 0) {
          const queryRes = await api.mpesa.query(reqId);
          if (queryRes && queryRes.paid) {
            clearInterval(pollIntervalRef.current);
            setPaymentSuccess(true);
            setConfirmedOrder(createdOrder);
            clearCart();
            setSubmitted(true);
            return;
          }
        }

        if (attempts >= maxAttempts) {
          clearInterval(pollIntervalRef.current);
          setStkLoading(false);
        }
      } catch (err) {
        console.warn('Polling check error:', err.message);
      }
    }, 3000);
  }

  // Trigger Lipa Na M-Pesa STK Push
  async function handleStkPush() {
    setStkLoading(true);
    setStkError('');
    setServerError('');

    try {
      // 1. Ensure order is created first so payment links directly to it
      const order = await ensureOrderCreated();
      const currentRef = order.order_ref;

      // 2. Trigger STK push
      const pushRes = await api.mpesa.stkPush({
        phone: formData.phone,
        amount: grandTotal,
        orderRef: currentRef,
        orderId: order.id,
      });

      if (pushRes && pushRes.checkoutRequestId) {
        setCheckoutRequestId(pushRes.checkoutRequestId);
        setStkSent(true);
        // 3. Start polling for confirmation
        startPaymentPolling(currentRef, pushRes.checkoutRequestId);
      } else {
        throw new Error(pushRes?.message || 'Failed to trigger M-Pesa request');
      }
    } catch (err) {
      setStkError(err.message || 'STK Push failed. Please try manual payment.');
    } finally {
      setStkLoading(false);
    }
  }

  // Final confirmation handler (for manual till payment or completing review)
  async function handleConfirm() {
    setSubmitting(true);
    setServerError('');
    try {
      const order = await ensureOrderCreated(mpesaCode.trim().toUpperCase());
      setConfirmedOrder(order);
      setSubmitted(true);
      clearCart();
    } catch (err) {
      setServerError(err.message || 'Order confirmation failed. Please try again.');
    } finally {
      setSubmitting(false);
    }
  }

  // ─── ORDER CONFIRMED SCREEN ───────────────────────────────────────────────
  if (submitted && confirmedOrder) {
    const isPaid = paymentSuccess || confirmedOrder.payment_status === 'paid';
    return (
      <Layout>
        <div className="max-w-2xl mx-auto px-4 py-16 text-center">
          <div className="inline-flex items-center justify-center w-20 h-20 bg-green-100 rounded-full mb-5">
            <CheckCircle className="w-12 h-12 text-green-600" />
          </div>

          <h1 className="font-display text-3xl font-extrabold text-gray-950 mb-2">Order Confirmed!</h1>
          <p className="text-gray-600 text-sm max-w-md mx-auto mb-6">
            Thank you, <strong className="text-gray-900">{formData.fullName}</strong>. Your order has been received and is being prepared.
          </p>

          {/* Order Details Card */}
          <div className="bg-white border border-gray-200 rounded-2xl p-6 text-left shadow-sm mb-6 space-y-4">
            <div className="flex flex-wrap items-center justify-between pb-4 border-b border-gray-100 gap-2">
              <div>
                <p className="text-[11px] font-bold uppercase tracking-wider text-gray-400">Order Reference</p>
                <p className="font-mono text-xl font-extrabold text-brand-600">{confirmedOrder.order_ref}</p>
              </div>
              <button
                onClick={() => copyOrderRef(confirmedOrder.order_ref)}
                className="flex items-center gap-1.5 px-3 py-1.5 bg-gray-100 hover:bg-gray-200 rounded-lg text-xs font-semibold text-gray-700 transition-colors"
              >
                {copiedRef ? <><Check className="w-3.5 h-3.5 text-green-600" /> Copied</> : <><Copy className="w-3.5 h-3.5" /> Copy Ref</>}
              </button>
            </div>

            <div className="grid grid-cols-2 gap-4 text-xs">
              <div>
                <span className="text-gray-400 block mb-0.5">Amount</span>
                <strong className="text-gray-900 font-bold text-sm">KSh {grandTotal.toLocaleString()}</strong>
              </div>
              <div>
                <span className="text-gray-400 block mb-0.5">Payment Status</span>
                <span className={`inline-flex items-center px-2 py-0.5 rounded font-bold ${
                  isPaid ? 'bg-green-100 text-green-800' : 'bg-yellow-100 text-yellow-800'
                }`}>
                  {isPaid ? '✓ Paid via M-Pesa' : 'Pending Verification'}
                </span>
              </div>
              <div>
                <span className="text-gray-400 block mb-0.5">Fulfillment</span>
                <strong className="text-gray-800">{formData.fulfillment === 'delivery' ? `Delivery (${formData.area})` : 'Store Pickup (Lurambi)'}</strong>
              </div>
              <div>
                <span className="text-gray-400 block mb-0.5">Contact Phone</span>
                <strong className="text-gray-800">{formData.phone}</strong>
              </div>
            </div>

            {isPaid && (
              <div className="p-3 bg-green-50 border border-green-200 rounded-xl text-xs text-green-800 flex items-center gap-2">
                <Check className="w-4 h-4 text-green-600 shrink-0" />
                <span>M-Pesa payment verified. Receipt sent to your phone.</span>
              </div>
            )}
          </div>

          <div className="flex flex-col sm:flex-row items-center justify-center gap-3">
            <Link
              to={`/account?tab=orders&ref=${confirmedOrder.order_ref}&phone=${formData.phone}`}
              className="btn-primary w-full sm:w-auto"
            >
              Track Order Status
            </Link>
            <Link
              to="/"
              className="btn-secondary w-full sm:w-auto"
            >
              Continue Shopping
            </Link>
          </div>
        </div>
      </Layout>
    );
  }

  // ─── CHECKOUT STEP FORM ───────────────────────────────────────────────────
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
        <div className="flex items-center w-full max-w-sm mb-8">
          {STEPS.map((label, i) => (
            <React.Fragment key={label}>
              <div className="flex flex-col items-center shrink-0">
                <div className={`w-8 h-8 rounded-full flex items-center justify-center text-xs font-bold transition-colors ${
                  step > i + 1 ? 'bg-brand-500 text-white' : step === i + 1 ? 'bg-brand-500 text-white ring-4 ring-brand-100' : 'bg-gray-100 text-gray-400'
                }`}>
                  {step > i + 1 ? <Check className="w-4 h-4" /> : i + 1}
                </div>
                <span className={`text-[11px] mt-1 ${step === i + 1 ? 'text-gray-900 font-medium' : 'text-gray-400'}`}>{label}</span>
              </div>
              {i < STEPS.length - 1 && (
                <div className={`flex-1 h-0.5 mx-2 mb-5 transition-colors ${step > i + 1 ? 'bg-brand-500' : 'bg-gray-200'}`} />
              )}
            </React.Fragment>
          ))}
        </div>

        <div className="grid lg:grid-cols-5 gap-8">
          <div className="lg:col-span-3 space-y-4">
            {serverError && (
              <div className="p-4 bg-red-50 text-red-700 rounded-control border border-red-200 text-sm font-semibold flex items-center gap-2">
                <AlertCircle className="w-4 h-4 shrink-0" />
                <span>{serverError}</span>
              </div>
            )}

            {/* STEP 1: Details */}
            {step === 1 && (
              <div className="bg-white border border-gray-200 rounded-card p-5 sm:p-6 shadow-sm space-y-5">
                <div>
                  <h2 className="font-display text-xl font-bold text-gray-900">Your Contact & Delivery Info</h2>
                  <p className="text-xs text-gray-400">Please provide accurate contact details for M-Pesa prompt and delivery.</p>
                </div>

                <div className="space-y-4">
                  <div>
                    <label className="text-xs font-bold uppercase text-gray-500 block mb-1.5">Full Name</label>
                    <input
                      type="text"
                      placeholder="e.g., Jane Naliaka"
                      value={formData.fullName}
                      onChange={(e) => setFormData({ ...formData, fullName: e.target.value })}
                      className={`w-full h-11 border rounded-control px-3.5 text-sm outline-none focus:border-brand-500 ${errors.fullName ? 'border-red-400' : 'border-gray-200'}`}
                    />
                    {errors.fullName && <p className="text-xs text-red-500 mt-1">{errors.fullName}</p>}
                  </div>
                  <div>
                    <label className="text-xs font-bold uppercase text-gray-500 block mb-1.5">M-Pesa Phone Number</label>
                    <input
                      type="tel"
                      placeholder="e.g., 0712345678"
                      value={formData.phone}
                      onChange={(e) => setFormData({ ...formData, phone: e.target.value })}
                      className={`w-full h-11 border rounded-control px-3.5 text-sm outline-none focus:border-brand-500 ${errors.phone ? 'border-red-400' : 'border-gray-200'}`}
                    />
                    {errors.phone && <p className="text-xs text-red-500 mt-1">{errors.phone}</p>}
                  </div>
                  <div>
                    <label className="text-xs font-bold uppercase text-gray-500 block mb-1.5">Email Address <span className="text-gray-300 font-normal normal-case">(for order confirmation)</span></label>
                    <input
                      type="email"
                      placeholder="e.g., yourname@gmail.com"
                      value={formData.email}
                      onChange={(e) => setFormData({ ...formData, email: e.target.value })}
                      className="w-full h-11 border border-gray-200 rounded-control px-3.5 text-sm outline-none focus:border-brand-500"
                    />
                  </div>
                </div>

                <hr className="border-gray-100" />

                <div>
                  <h2 className="font-display text-xl font-bold text-gray-900">Fulfillment Method</h2>
                  <p className="text-xs text-gray-400">Choose how you want to receive your items.</p>
                </div>

                <div className="grid grid-cols-2 gap-4">
                  {[
                    { type: 'delivery', Icon: Truck, label: 'Delivery', sub: 'Sun–Fri 8:30am–4:00pm' },
                    { type: 'pickup', Icon: Store, label: 'Pickup', sub: 'Ready in 1–2 hours' },
                  ].map(({ type, Icon, label, sub }) => (
                    <button
                      key={type}
                      type="button"
                      onClick={() => setFormData({ ...formData, fulfillment: type })}
                      className={`h-24 rounded-card border-2 flex flex-col items-center justify-center gap-1.5 transition-colors ${
                        formData.fulfillment === type ? 'border-brand-500 bg-brand-50/50' : 'border-gray-200 hover:border-gray-300'
                      }`}
                    >
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
                      <input
                        type="text"
                        placeholder="e.g., Lurambi, Amalemba, Milimani, Kefinco"
                        value={formData.area}
                        onChange={(e) => setFormData({ ...formData, area: e.target.value })}
                        className={`w-full h-11 border rounded-control px-3.5 text-sm outline-none focus:border-brand-500 ${errors.area ? 'border-red-400' : 'border-gray-200'}`}
                      />
                      {errors.area && <p className="text-xs text-red-500 mt-1">{errors.area}</p>}
                    </div>
                    <div>
                      <label className="text-xs font-bold uppercase text-gray-500 block mb-1.5">Directions / Landmark (Optional)</label>
                      <textarea
                        placeholder="e.g., Near Lurambi stage, opposite the green pharmacy"
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
                      <p className="text-xs text-gray-400 mt-1">Hours: Sun–Thu 8:30am–8:00pm · Fri 8:30am–3:00pm · Closed Saturday</p>
                    </div>
                  </div>
                )}

                <button
                  onClick={() => {
                    if (validateStep1()) setStep(2);
                  }}
                  className="btn-primary flex items-center gap-2 cursor-pointer"
                >
                  Continue to Payment <ChevronRight className="w-4 h-4" />
                </button>
              </div>
            )}

            {/* STEP 2: M-Pesa STK Push */}
            {step === 2 && (
              <div className="bg-white border border-gray-200 rounded-card p-5 sm:p-6 shadow-sm space-y-5">
                <div>
                  <h2 className="font-display text-xl font-bold text-gray-900">Pay via M-Pesa</h2>
                  <p className="text-xs text-gray-400">Trigger an instant prompt to your phone or pay directly to our Till.</p>
                </div>

                {/* Amount callout */}
                <div className="bg-brand-50 border border-brand-200 rounded-card p-4 flex justify-between items-center">
                  <div>
                    <p className="text-[10px] text-brand-700 font-bold uppercase">Total Payable</p>
                    <p className="font-extrabold text-2xl text-brand-700">KSh {grandTotal.toLocaleString()}</p>
                  </div>
                  <div className="text-right">
                    <p className="text-[10px] text-gray-400 uppercase font-bold">Buy Goods Till</p>
                    <p className="font-mono font-extrabold text-2xl text-gray-900">{TILL_NUMBER}</p>
                    <button onClick={copyTillNumber} className="mt-1 flex items-center gap-1 text-xs text-brand-500 ml-auto cursor-pointer">
                      {copiedTill ? <><Check className="w-3.5 h-3.5 text-green-600" /> Copied!</> : <><Copy className="w-3.5 h-3.5" /> Copy Till</>}
                    </button>
                  </div>
                </div>

                {/* STK Push section */}
                {!stkSent ? (
                  <div className="space-y-4">
                    <div className="bg-surface rounded-card p-4 border border-gray-200 text-xs text-gray-600 space-y-2">
                      <p className="font-bold text-gray-800">Automatic STK Push:</p>
                      <p>1. Click <strong>Send M-Pesa Prompt</strong> below.</p>
                      <p>2. A PIN prompt will pop up on <strong>{formData.phone}</strong>.</p>
                      <p>3. Enter your M-Pesa PIN to complete payment of KSh {grandTotal.toLocaleString()}.</p>
                    </div>

                    {stkError && <div className="p-3 bg-red-50 border border-red-200 text-red-700 text-xs rounded-xl font-medium">{stkError}</div>}

                    <button
                      onClick={handleStkPush}
                      disabled={stkLoading}
                      className="w-full h-12 bg-green-600 hover:bg-green-700 disabled:opacity-70 text-white rounded-xl font-bold text-sm flex items-center justify-center gap-2 transition-colors cursor-pointer"
                    >
                      {stkLoading ? <><Loader className="w-4 h-4 animate-spin" /> Contacting Safaricom…</> : <><Smartphone className="w-4 h-4" /> Send M-Pesa Prompt to {formData.phone}</>}
                    </button>
                    <p className="text-center text-xs text-gray-400">Or pay manually using the Till number above, then enter the code below.</p>
                  </div>
                ) : (
                  // Waiting screen after STK sent
                  <div className="space-y-4">
                    <div className="bg-green-50 border border-green-200 rounded-xl p-5 text-center space-y-3">
                      <Loader className="w-8 h-8 text-green-600 animate-spin mx-auto" />
                      <p className="font-bold text-green-800">Prompt Sent to {formData.phone}!</p>
                      <p className="text-xs text-green-700">Check your phone screen, enter your M-Pesa PIN, and this page will confirm automatically.</p>
                    </div>
                    <button
                      onClick={() => {
                        setStkSent(false);
                        setStkError('');
                      }}
                      className="text-xs text-gray-500 hover:text-gray-700 underline w-full text-center cursor-pointer"
                    >
                      Didn't receive the prompt? Resend or pay manually
                    </button>
                  </div>
                )}

                {/* Manual code fallback — always visible */}
                <div className="pt-2">
                  <label className="text-xs font-bold uppercase text-gray-500 block mb-1.5">
                    M-Pesa Transaction Code <span className="text-gray-400 normal-case font-normal">(if you paid manually via Till {TILL_NUMBER})</span>
                  </label>
                  <input
                    type="text"
                    placeholder="e.g., RFT34HG76D"
                    value={mpesaCode}
                    onChange={(e) => setMpesaCode(e.target.value.toUpperCase())}
                    className="w-full h-11 border border-gray-200 rounded-control px-3.5 text-sm font-mono outline-none focus:border-brand-500 uppercase tracking-wider"
                  />
                </div>

                <div className="flex gap-3 pt-2">
                  <button onClick={() => setStep(3)} className="btn-primary flex items-center gap-2 cursor-pointer">
                    Review Order <ChevronRight className="w-4 h-4" />
                  </button>
                  <button onClick={() => setStep(1)} className="btn-secondary cursor-pointer">Back</button>
                </div>
              </div>
            )}

            {/* STEP 3: Review & Confirm */}
            {step === 3 && (
              <div className="bg-white border border-gray-200 rounded-card p-5 sm:p-6 shadow-sm space-y-5">
                <div>
                  <h2 className="font-display text-xl font-bold text-gray-900">Review & Confirm</h2>
                  <p className="text-xs text-gray-400">Please review your order details before final submission.</p>
                </div>

                <div className="grid grid-cols-1 sm:grid-cols-2 gap-4">
                  <div className="bg-surface rounded-card p-4 border border-gray-200">
                    <p className="text-[10px] text-gray-400 font-bold uppercase mb-2">Customer & Destination</p>
                    <p className="text-sm font-bold text-gray-800">{formData.fullName}</p>
                    <p className="text-xs text-gray-500">{formData.phone}</p>
                    <p className="text-xs text-gray-600 mt-2">
                      {formData.fulfillment === 'delivery' ? `Delivery to: ${formData.area}` : 'Physical Store Pickup (Lurambi)'}
                    </p>
                  </div>
                  <div className="bg-surface rounded-card p-4 border border-gray-200">
                    <p className="text-[10px] text-gray-400 font-bold uppercase mb-2">Payment Details</p>
                    <p className="text-xs text-gray-600">Method: Lipa Na M-Pesa</p>
                    {mpesaCode ? (
                      <p className="text-xs font-mono font-bold text-brand-600 mt-1">Transaction Code: {mpesaCode}</p>
                    ) : stkSent ? (
                      <p className="text-xs text-green-600 font-semibold mt-1">✓ STK prompt triggered to phone</p>
                    ) : (
                      <p className="text-xs text-gray-500 mt-1">Till Number: {TILL_NUMBER}</p>
                    )}
                  </div>
                </div>

                <div className="border border-gray-100 rounded-card overflow-hidden">
                  <div className="bg-surface px-4 py-2 border-b border-gray-100 text-[10px] uppercase font-bold text-gray-500">Order Items</div>
                  {items.map((item) => (
                    <div key={item.product.id} className="flex justify-between items-center px-4 py-3 text-sm border-b border-gray-50 last:border-0">
                      <span className="text-gray-700">{item.product.name} × {item.quantity}</span>
                      <strong className="text-gray-900">KSh {(item.product.price * item.quantity).toLocaleString()}</strong>
                    </div>
                  ))}
                </div>

                <div className="flex gap-3 pt-2">
                  <button onClick={handleConfirm} disabled={submitting} className="btn-primary flex items-center justify-center gap-2 min-w-44 cursor-pointer">
                    {submitting ? 'Placing Order...' : 'Place Order'} {!submitting && <CheckCircle className="w-4 h-4" />}
                  </button>
                  <button onClick={() => setStep(2)} className="btn-secondary cursor-pointer">Back</button>
                </div>
              </div>
            )}
          </div>

          {/* Order Summary sidebar */}
          <div className="lg:col-span-2">
            <div className="bg-white border border-gray-200 rounded-card p-5 sticky top-28 shadow-sm space-y-4">
              <h3 className="font-display text-lg font-bold text-gray-900 border-b border-gray-100 pb-2">Order Overview</h3>
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
                <div className="flex justify-between text-gray-500"><span>Delivery Fee</span><strong className="text-gray-800">{formData.fulfillment === 'delivery' ? (isFreeDelivery(total) ? 'Free' : `KSh ${deliveryFee.toLocaleString()}`) : 'KSh 0'}</strong></div>
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
