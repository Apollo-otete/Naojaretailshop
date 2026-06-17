import { Link } from 'react-router-dom';

export default function Footer() {
  return (
    <footer className="border-t border-gray-200 bg-white py-12">
      <div className="max-w-7xl mx-auto px-4">
        <div className="grid grid-cols-1 md:grid-cols-3 gap-8">
          <div>
            <Link to="/" className="font-serif font-semibold text-2xl">
              Naoja <span className="text-brand-500">.</span>
            </Link>
            <p className="mt-2 text-sm text-gray-500">
              Pickup: Kakamega, Lurambi, Opposite Bamboo. Closed Saturdays.
            </p>
          </div>

          <div>
            <h4 className="font-semibold text-gray-900 mb-3">Delivery & Pickup</h4>
            <p className="text-sm text-gray-500">Free delivery from KSh 600</p>
            <p className="text-sm text-gray-500">Delivery: Sun-Fri 8am-4pm</p>
            <p className="text-sm text-gray-500">Pickup: Sun-Thu 8am-8pm, Fri 8am-4pm</p>
          </div>

          <div>
            <h4 className="font-semibold text-gray-900 mb-3">Support</h4>
            <p className="text-sm text-gray-500">Payments: M-Pesa</p>
            <p className="text-sm text-gray-500">Warranty & returns available</p>
          </div>
        </div>

        <div className="mt-8 pt-8 border-t border-gray-200 text-center text-sm text-gray-500">
          <p>&copy; {new Date().getFullYear()} Naoja Electrical & Electronics. All rights reserved.</p>
        </div>
      </div>
    </footer>
  );
}
