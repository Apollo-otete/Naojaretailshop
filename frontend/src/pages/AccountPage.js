import { Link, useSearchParams } from 'react-router-dom';
import { useState } from 'react';
import { Package, MapPin, Truck, Headphones, Send, Clock, CheckCircle } from 'lucide-react';
import Layout from '../components/Layout';
import { api } from '../lib/api';

const TABS = [
  { id: 'orders', label: 'Orders', icon: Package },
  { id: 'location', label: 'Store Location', icon: MapPin },
  { id: 'policy', label: 'Delivery & Pickup', icon: Truck },
  { id: 'support', label: 'Support & Contact', icon: Headphones },
];

export default function AccountPage() {
  const [searchParams, setSearchParams] = useSearchParams();
  const activeTab = searchParams.get('tab') || 'orders';

  // Contact form state
  const [contactName, setContactName] = useState('');
  const [contactEmail, setContactEmail] = useState('');
  const [contactMessage, setContactMessage] = useState('');
  const [contactStatus, setContactStatus] = useState({ success: false, message: '' });
  const [submitting, setSubmitting] = useState(false);

  const setTab = (tab) => {
    setSearchParams({ tab });
  };

  const handleContactSubmit = async (e) => {
    e.preventDefault();
    if (!contactName || !contactEmail || !contactMessage) return;
    setSubmitting(true);
    setContactStatus({ success: false, message: '' });
    try {
      const res = await api.submitContactForm({
        name: contactName,
        email: contactEmail,
        message: contactMessage,
      });
      setContactStatus({
        success: true,
        message: res.message || 'Thank you! Your message has been sent successfully. We will get back to you shortly.',
      });
      setContactName('');
      setContactEmail('');
      setContactMessage('');
    } catch (error) {
      setContactStatus({
        success: false,
        message: error.message || 'Failed to send your message. Please try again.',
      });
    } finally {
      setSubmitting(false);
    }
  };

  return (
    <Layout>
      <div className="max-w-7xl mx-auto px-4 py-8">
        <div className="grid lg:grid-cols-[260px_1fr] gap-8">
          {/* Sidebar */}
          <aside className="bg-white border border-gray-200 rounded-card p-5 h-fit shadow-sm">
            <h2 className="font-extrabold text-gray-900 mb-4 px-2 text-base uppercase tracking-wider text-gray-400 text-xs">My Account</h2>
            <nav className="space-y-1">
              {TABS.map((tab) => {
                const Icon = tab.icon;
                return (
                  <button
                    key={tab.id}
                    onClick={() => setTab(tab.id)}
                    className={`w-full flex items-center gap-3 px-4 py-2.5 rounded-control text-left transition-colors cursor-pointer text-sm font-semibold ${
                      activeTab === tab.id
                        ? 'bg-brand-50 text-brand-600'
                        : 'text-gray-600 hover:bg-surface'
                    }`}
                  >
                    <Icon className="w-4 h-4 shrink-0" />
                    <span>{tab.label}</span>
                  </button>
                );
              })}
            </nav>
          </aside>

          {/* Content */}
          <main className="bg-white border border-gray-200 rounded-card p-6 sm:p-8 shadow-sm">
            {/* Orders Tab */}
            {activeTab === 'orders' && (
              <section className="space-y-6">
                <div className="border-b border-gray-100 pb-4">
                  <h2 className="font-serif text-3xl font-bold text-gray-900">Track Orders</h2>
                  <p className="text-xs text-gray-400 mt-1">Review the status of your current and past orders.</p>
                </div>
                <div className="bg-surface rounded-card p-12 text-center border border-gray-200">
                  <Package className="w-14 h-14 text-gray-300 mx-auto mb-4" />
                  <p className="text-gray-500 text-sm">You have no active orders placed under this session.</p>
                  <p className="text-xs text-gray-400 mt-1.5">For enquiries regarding an M-Pesa order, please call us directly.</p>
                  <Link to="/" className="btn-secondary inline-flex mt-6 text-sm">
                    Start Shopping
                  </Link>
                </div>
              </section>
            )}

            {/* Store Location Tab */}
            {activeTab === 'location' && (
              <section className="space-y-6">
                <div className="border-b border-gray-100 pb-4">
                  <h2 className="font-serif text-3xl font-bold text-gray-900">Store Location (Kakamega)</h2>
                  <p className="text-xs text-gray-400 mt-1">Visit our retail shop for physical support, browsing, or order pickup.</p>
                </div>
                
                <div className="grid md:grid-cols-2 gap-6">
                  <div className="bg-surface rounded-card p-5 border border-gray-200 space-y-3">
                    <h3 className="font-bold text-gray-900 text-base">Address</h3>
                    <p className="text-sm text-gray-600">
                      Naoja Ventures<br />
                      Lurambi, Kakamega Town<br />
                      Opposite Bamboo Hotel / Lounge<br />
                      Kenya
                    </p>
                    <div className="pt-2">
                      <p className="text-xs font-bold uppercase text-gray-400 mb-1">Contact Info</p>
                      <p className="text-sm text-gray-700">Phone: <strong>0704812343</strong></p>
                    </div>
                  </div>

                  <div className="bg-surface rounded-card p-5 border border-gray-200 space-y-3">
                    <h3 className="font-bold text-gray-900 text-base">Shop Working Hours</h3>
                    <div className="text-sm text-gray-600 space-y-1">
                      <p className="flex justify-between"><span>Sunday - Thursday:</span> <strong>8:30 AM - 8:00 PM</strong></p>
                      <p className="flex justify-between"><span>Friday:</span> <strong>8:30 AM - 3:00 PM</strong></p>
                      <p className="flex justify-between text-red-600 font-semibold"><span>Saturday:</span> <strong>Closed</strong></p>
                    </div>
                  </div>
                </div>

                <div className="bg-gray-100 rounded-card h-64 flex flex-col items-center justify-center border border-gray-200 text-center p-6">
                  <MapPin className="w-10 h-10 text-brand-500 mb-2" />
                  <p className="font-bold text-gray-800">Map Location</p>
                  <p className="text-xs text-gray-500 mt-1 max-w-xs">
                    Located along the main Kakamega-Webuye highway, right opposite the famous Bamboo Lounge/Hotel in Lurambi.
                  </p>
                </div>
              </section>
            )}

            {/* Delivery & Pickup Policies Tab */}
            {activeTab === 'policy' && (
              <section className="space-y-6">
                <div className="border-b border-gray-100 pb-4">
                  <h2 className="font-serif text-3xl font-bold text-gray-900">Fulfillment Policy</h2>
                  <p className="text-xs text-gray-400 mt-1">Review rates, timings, and instructions for deliveries and pickups.</p>
                </div>

                <div className="grid md:grid-cols-2 gap-6">
                  <div className="bg-surface rounded-card p-5 border border-gray-200 space-y-2">
                    <div className="flex items-center gap-2 text-brand-700">
                      <Truck className="w-5 h-5" />
                      <h3 className="font-bold text-base">Delivery Information</h3>
                    </div>
                    <div className="text-sm text-gray-600 space-y-2 pt-2">
                      <p>• We deliver within Kakamega town and surrounding neighborhoods (Lurambi, Kefinco, Amalemba, Joyland, Milimani).</p>
                      <p>• Delivery fee is flat **KSh 150**. Orders **above KSh 600** qualify for **Free Delivery**.</p>
                      <p>• Delivery hours: Sunday-Friday, 8:30 AM - 4:00 PM.</p>
                      <p>• Orders before 4:00 PM are delivered same-day. Orders after 4:00 PM go out the next morning.</p>
                    </div>
                  </div>

                  <div className="bg-surface rounded-card p-5 border border-gray-200 space-y-2">
                    <div className="flex items-center gap-2 text-brand-700">
                      <Clock className="w-5 h-5" />
                      <h3 className="font-bold text-base">Store Pickups</h3>
                    </div>
                    <div className="text-sm text-gray-600 space-y-2 pt-2">
                      <p>• You can place an order and pick it up physically from our Lurambi store opposite Bamboo.</p>
                      <p>• Pickups are ready within **1 to 2 hours** of ordering during business hours.</p>
                      <p>• Pickup is completely free of charge.</p>
                      <p>• Please bring your order name and M-Pesa reference message to the counter for verification.</p>
                    </div>
                  </div>
                </div>
              </section>
            )}

            {/* Support Tab with Contact Form */}
            {activeTab === 'support' && (
              <section className="space-y-6">
                <div className="border-b border-gray-100 pb-4">
                  <h2 className="font-serif text-3xl font-bold text-gray-900">Support &amp; Inquiries</h2>
                  <p className="text-xs text-gray-400 mt-1">Contact us regarding orders, brand partnerships, or general technical support.</p>
                </div>

                <div className="grid md:grid-cols-12 gap-8">
                  {/* Left: Contact Info */}
                  <div className="md:col-span-5 space-y-5">
                    <div className="bg-surface rounded-card p-5 border border-gray-200 space-y-4">
                      <div>
                        <h4 className="font-bold text-gray-800 text-sm">Direct Support lines</h4>
                        <p className="text-xs text-gray-500 mt-1">Our phone support is open 8:30am-8:00pm (Sunday-Thursday).</p>
                      </div>
                      <div className="text-sm text-gray-600 space-y-2.5">
                        <p>📞 Phone: <a href="tel:0704812343" className="text-brand-500 font-semibold hover:underline">0704812343</a></p>
                        <p>💬 WhatsApp: <a href="https://wa.me/254704812343" target="_blank" rel="noopener noreferrer" className="text-brand-500 font-semibold hover:underline">0704812343</a></p>
                        <p>✉️ Email: <a href="mailto:enquiries@naojaventures.com" className="text-brand-500 font-semibold hover:underline">enquiries@naojaventures.com</a></p>
                      </div>
                    </div>

                    <div className="bg-surface rounded-card p-5 border border-gray-200 space-y-2">
                      <h4 className="font-bold text-gray-800 text-sm">Warranty Policy</h4>
                      <p className="text-xs text-gray-600 leading-relaxed">
                        All electrical supplies (cables, breakers, solar) and electronics sold at Naoja Ventures have official manufacturer warranties. Please retain invoice receipts for verification.
                      </p>
                    </div>
                  </div>

                  {/* Right: Contact Form */}
                  <div className="md:col-span-7 bg-white border border-gray-200 rounded-card p-5 sm:p-6 shadow-sm">
                    <h3 className="font-serif text-xl font-bold text-gray-900 mb-1">Send an Inquiry</h3>
                    <p className="text-xs text-gray-500 mb-5">Have a question or request? Send us a message and we'll reply via email.</p>

                    {contactStatus.message && (
                      <div className={`p-4 rounded-control text-xs font-semibold mb-5 flex items-start gap-2 ${
                        contactStatus.success ? 'bg-green-50 text-green-700 border border-green-200' : 'bg-red-50 text-red-700 border border-red-200'
                      }`}>
                        {contactStatus.success && <CheckCircle className="w-4 h-4 text-green-600 shrink-0 mt-0.5" />}
                        <span>{contactStatus.message}</span>
                      </div>
                    )}

                    <form onSubmit={handleContactSubmit} className="space-y-4">
                      <div className="grid grid-cols-1 sm:grid-cols-2 gap-4">
                        <div>
                          <label className="text-xs font-bold uppercase text-gray-500 block mb-1.5">Your Name</label>
                          <input
                            type="text"
                            required
                            value={contactName}
                            onChange={(e) => setContactName(e.target.value)}
                            placeholder="Enter your name"
                            className="w-full h-11 border border-gray-200 rounded-control px-3.5 text-sm focus:border-brand-500 outline-none"
                          />
                        </div>
                        <div>
                          <label className="text-xs font-bold uppercase text-gray-500 block mb-1.5">Email Address</label>
                          <input
                            type="email"
                            required
                            value={contactEmail}
                            onChange={(e) => setContactEmail(e.target.value)}
                            placeholder="name@example.com"
                            className="w-full h-11 border border-gray-200 rounded-control px-3.5 text-sm focus:border-brand-500 outline-none"
                          />
                        </div>
                      </div>

                      <div>
                        <label className="text-xs font-bold uppercase text-gray-500 block mb-1.5">Message</label>
                        <textarea
                          required
                          rows={4}
                          value={contactMessage}
                          onChange={(e) => setContactMessage(e.target.value)}
                          placeholder="How can we help you?"
                          className="w-full border border-gray-200 rounded-control p-3.5 text-sm focus:border-brand-500 outline-none resize-none"
                        />
                      </div>

                      <button
                        type="submit"
                        disabled={submitting}
                        className="btn-primary w-full flex items-center justify-center gap-2 cursor-pointer"
                      >
                        {submitting ? 'Sending...' : 'Send Message'}
                        {!submitting && <Send className="w-4 h-4" />}
                      </button>
                    </form>
                  </div>
                </div>
              </section>
            )}
          </main>
        </div>
      </div>
    </Layout>
  );
}
