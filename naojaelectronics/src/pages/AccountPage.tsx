import { Link, useSearchParams } from 'react-router-dom';
import { Package, MapPin, Truck, Headphones } from 'lucide-react';
import Layout from '../components/Layout';

const TABS = [
  { id: 'orders', label: 'Orders', icon: Package },
  { id: 'location', label: 'Store Location', icon: MapPin },
  { id: 'policy', label: 'Delivery & Pickup', icon: Truck },
  { id: 'support', label: 'Support', icon: Headphones },
];

export default function AccountPage() {
  const [searchParams, setSearchParams] = useSearchParams();
  const activeTab = searchParams.get('tab') || 'orders';

  const setTab = (tab: string) => {
    setSearchParams({ tab });
  };

  return (
    <Layout>
      <div className="max-w-7xl mx-auto px-4 py-8">
        <div className="grid lg:grid-cols-[280px_1fr] gap-6">
          {/* Sidebar */}
          <aside className="bg-white border border-gray-200 rounded-card p-4">
            <h2 className="font-extrabold text-gray-900 mb-3">My Account</h2>
            <nav className="space-y-2">
              {TABS.map((tab) => (
                <button
                  key={tab.id}
                  onClick={() => setTab(tab.id)}
                  className={`w-full flex items-center gap-2.5 px-3 py-2 rounded-control text-left transition-colors ${
                    activeTab === tab.id
                      ? 'bg-brand-50 text-brand-600'
                      : 'text-gray-500 hover:bg-surface'
                  }`}
                >
                  <tab.icon className="w-4 h-4" />
                  <span className="text-sm">{tab.label}</span>
                </button>
              ))}
            </nav>
          </aside>

          {/* Content */}
          <main className="bg-white border border-gray-200 rounded-card p-6">
            {/* Orders */}
            {activeTab === 'orders' && (
              <section>
                <h2 className="font-serif text-3xl text-gray-900 mb-4">Orders</h2>
                <div className="bg-surface rounded-card p-8 text-center">
                  <Package className="w-12 h-12 text-gray-300 mx-auto mb-3" />
                  <p className="text-gray-500">You have no orders yet.</p>
                  <Link to="/" className="btn-secondary inline-flex mt-4">
                    Start Shopping
                  </Link>
                </div>
              </section>
            )}

            {/* Store Location */}
            {activeTab === 'location' && (
              <section>
                <h2 className="font-serif text-3xl text-gray-900 mb-4">Store Location (Pickup)</h2>
                <div className="bg-surface rounded-card p-4">
                  <p className="font-bold text-gray-900">Kakamega, Lurambi, Opposite Bamboo</p>
                  <p className="text-gray-500 text-sm mt-2">
                    Pickup ready in 1-2 hours (during shop hours). Closed Saturdays.
                  </p>
                  <p className="text-gray-500 text-sm mt-3">
                    Shop hours: Sun-Thu 8am-8pm - Fri 8am-4pm
                  </p>
                </div>

                <div className="mt-6 h-64 bg-gray-100 rounded-card flex items-center justify-center text-gray-400">
                  Map placeholder
                </div>
              </section>
            )}

            {/* Delivery & Pickup Policy */}
            {activeTab === 'policy' && (
              <section>
                <h2 className="font-serif text-3xl text-gray-900 mb-4">Delivery & Pickup Policy</h2>

                <div className="bg-surface rounded-card p-4 mb-4">
                  <h3 className="font-bold text-gray-900 mb-2">Delivery</h3>
                  <div className="text-sm text-gray-500 space-y-1">
                    <p>Days: Sunday-Friday</p>
                    <p>Hours: 8:00am-4:00pm</p>
                    <p>Cutoff: Order before 4:00pm for same-day delivery; after 4:00pm next-day</p>
                    <p>Free delivery: Orders with subtotal &ge; KSh 600</p>
                    <p>Saturday: Closed (no delivery)</p>
                  </div>
                </div>

                <div className="bg-surface rounded-card p-4 mb-4">
                  <h3 className="font-bold text-gray-900 mb-2">Pickup</h3>
                  <div className="text-sm text-gray-500 space-y-1">
                    <p>Location: Kakamega, Lurambi, Opposite Bamboo</p>
                    <p>Ready time: 1-2 hours after ordering (during open hours)</p>
                    <p>Hours: Sun-Thu 8am-8pm - Fri 8am-4pm</p>
                    <p>Saturday: Closed</p>
                  </div>
                </div>

                <div className="bg-surface rounded-card p-4">
                  <h3 className="font-bold text-gray-900 mb-2">Payment</h3>
                  <div className="text-sm text-gray-500">
                    <p>All payments are processed via M-Pesa.</p>
                    <p className="mt-2">You will receive an M-Pesa push notification to confirm payment.</p>
                  </div>
                </div>
              </section>
            )}

            {/* Support */}
            {activeTab === 'support' && (
              <section>
                <h2 className="font-serif text-3xl text-gray-900 mb-4">Support</h2>

                <div className="bg-surface rounded-card p-4 mb-4">
                  <h3 className="font-bold text-gray-900 mb-2">Payments</h3>
                  <p className="text-sm text-gray-500">
                    All payments are processed securely via M-Pesa. You'll receive a payment prompt on your phone to confirm.
                  </p>
                </div>

                <div className="bg-surface rounded-card p-4 mb-4">
                  <h3 className="font-bold text-gray-900 mb-2">Warranty & Returns</h3>
                  <p className="text-sm text-gray-500">
                    All products come with manufacturer warranty support. For warranty claims or returns, please visit our store or contact us.
                  </p>
                </div>

                <div className="bg-surface rounded-card p-4">
                  <h3 className="font-bold text-gray-900 mb-2">Contact Us</h3>
                  <p className="text-sm text-gray-500">
                    Visit us at: Kakamega, Lurambi, Opposite Bamboo
                  </p>
                  <p className="text-sm text-gray-500 mt-2">
                    Open: Sun-Thu 8am-8pm, Fri 8am-4pm
                  </p>
                  <p className="text-sm text-gray-500 mt-1">
                    Closed on Saturdays
                  </p>
                </div>
              </section>
            )}
          </main>
        </div>
      </div>
    </Layout>
  );
}
