import { Link } from 'react-router-dom';

export default function Footer() {
  return (
    <footer className="border-t border-gray-200 bg-white py-12">
      <div className="max-w-7xl mx-auto px-4">
        <div className="grid grid-cols-1 md:grid-cols-4 gap-8">
          <div className="md:col-span-2">
            <Link to="/" className="font-serif font-semibold text-2xl">
              Naoja <span className="text-brand-500">.</span>
            </Link>
            <p className="mt-2 text-sm text-gray-500 max-w-sm">
              Your Trusted Electrical & Electronics Partner in Kakamega. Providing genuine products, convenient payments, and fast fulfillment.
            </p>
            <p className="mt-4 text-xs text-gray-400">
              Location: Lurambi, Kakamega, Opposite Bamboo.
            </p>
          </div>

          <div>
            <h4 className="font-semibold text-gray-900 mb-3">Business Hours</h4>
            <p className="text-sm text-gray-500">Sunday – Thursday: 8:30 AM – 8:00 PM</p>
            <p className="text-sm text-gray-500">Friday: 8:30 AM – 3:00 PM</p>
            <p className="text-sm text-gray-500 font-semibold text-red-600">Saturday: Closed</p>
          </div>

          <div>
            <h4 className="font-semibold text-gray-900 mb-3">Quick Support</h4>
            <p className="text-sm text-gray-500">Call/SMS: <a href="tel:0704812343" className="hover:text-brand-500 font-medium">0704812343</a></p>
            <p className="text-sm text-gray-500">WhatsApp: <a href="https://wa.me/254704812343" target="_blank" rel="noopener noreferrer" className="hover:text-brand-500 font-medium">0704812343</a></p>
            <p className="text-sm text-gray-500">Email: <a href="mailto:enquiries@naojaventures.com" className="hover:text-brand-500 font-medium">enquiries@naojaventures.com</a></p>
          </div>
        </div>

        <div className="mt-8 pt-8 border-t border-gray-200 flex flex-col sm:flex-row items-center justify-between text-sm text-gray-500">
          <p>&copy; {new Date().getFullYear()} Naoja Ventures. All rights reserved.</p>
          <div className="flex gap-4 mt-4 sm:mt-0">
            <Link to="/admin" className="text-xs text-gray-400 hover:text-brand-500">Admin Portal</Link>
          </div>
        </div>
      </div>
    </footer>
  );
}
