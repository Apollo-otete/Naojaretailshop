import { Link } from 'react-router-dom';
import { MapPin, Phone, Mail, Clock, ShieldCheck, Truck, CreditCard, Award } from 'lucide-react';

export default function Footer() {
  return (
    <footer className="bg-gray-900 text-gray-300">
      {/* Trust Strip */}
      <div className="border-b border-gray-800">
        <div className="max-w-7xl mx-auto px-4 py-5 grid grid-cols-2 md:grid-cols-4 gap-4">
          <div className="flex items-center gap-3">
            <div className="w-10 h-10 rounded-full bg-brand-500/15 flex items-center justify-center shrink-0">
              <Truck className="w-5 h-5 text-brand-400" />
            </div>
            <div>
              <p className="text-xs font-bold text-white">Same-Day Delivery</p>
              <p className="text-[10px] text-gray-500">Within Kakamega & Lurambi</p>
            </div>
          </div>
          <div className="flex items-center gap-3">
            <div className="w-10 h-10 rounded-full bg-emerald-500/15 flex items-center justify-center shrink-0">
              <CreditCard className="w-5 h-5 text-emerald-400" />
            </div>
            <div>
              <p className="text-xs font-bold text-white">Lipa Na M-Pesa</p>
              <p className="text-[10px] text-gray-500">Till: 4149288</p>
            </div>
          </div>
          <div className="flex items-center gap-3">
            <div className="w-10 h-10 rounded-full bg-blue-500/15 flex items-center justify-center shrink-0">
              <ShieldCheck className="w-5 h-5 text-blue-400" />
            </div>
            <div>
              <p className="text-xs font-bold text-white">100% Genuine</p>
              <p className="text-[10px] text-gray-500">Warranty on all products</p>
            </div>
          </div>
          <div className="flex items-center gap-3">
            <div className="w-10 h-10 rounded-full bg-amber-500/15 flex items-center justify-center shrink-0">
              <Award className="w-5 h-5 text-amber-400" />
            </div>
            <div>
              <p className="text-xs font-bold text-white">Trusted Since 2020</p>
              <p className="text-[10px] text-gray-500">Kakamega's #1 electronics</p>
            </div>
          </div>
        </div>
      </div>

      {/* Main Footer Content */}
      <div className="max-w-7xl mx-auto px-4 py-10">
        <div className="grid grid-cols-1 md:grid-cols-4 gap-8">
          {/* Brand */}
          <div className="md:col-span-1">
            <Link to="/" className="flex items-center gap-2">
              <div className="w-9 h-9 rounded-lg bg-brand-500 flex items-center justify-center text-white font-black text-lg">N</div>
              <div>
                <span className="font-black text-lg text-white tracking-tight">NAOJA</span>
                <span className="text-[9px] text-brand-400 font-bold uppercase block -mt-1 tracking-wider">Ventures Kakamega</span>
              </div>
            </Link>
            <p className="mt-3 text-xs text-gray-500 leading-relaxed">
              Your trusted electrical & electronics partner in Western Kenya. Genuine products, competitive prices, and reliable M-Pesa payments.
            </p>
          </div>

          {/* Categories */}
          <div>
            <h4 className="font-bold text-white text-sm mb-4">Shop by Category</h4>
            <div className="space-y-2">
              {[
                { name: 'Mobile Phones', slug: 'mobile-phones' },
                { name: 'Solar & Energy', slug: 'renewable-energy' },
                { name: 'TVs & Audio', slug: 'television-products' },
                { name: 'Computers', slug: 'computers-accessories' },
                { name: 'Security CCTV', slug: 'security-products' },
                { name: 'Electrical', slug: 'electrical-products' },
              ].map((cat) => (
                <Link key={cat.slug} to={`/category/${cat.slug}`} className="block text-xs text-gray-400 hover:text-brand-400 transition-colors">
                  {cat.name}
                </Link>
              ))}
            </div>
          </div>

          {/* Business Hours */}
          <div>
            <h4 className="font-bold text-white text-sm mb-4">Business Hours</h4>
            <div className="space-y-2 text-xs text-gray-400">
              <div className="flex items-center gap-2">
                <Clock className="w-3.5 h-3.5 text-brand-400" />
                <span>Sun – Thu: 8:30 AM – 8:00 PM</span>
              </div>
              <div className="flex items-center gap-2">
                <Clock className="w-3.5 h-3.5 text-brand-400" />
                <span>Friday: 8:30 AM – 3:00 PM</span>
              </div>
              <div className="flex items-center gap-2">
                <Clock className="w-3.5 h-3.5 text-red-400" />
                <span className="text-red-400 font-semibold">Saturday: Closed</span>
              </div>
            </div>
          </div>

          {/* Contact */}
          <div>
            <h4 className="font-bold text-white text-sm mb-4">Contact Us</h4>
            <div className="space-y-2.5 text-xs text-gray-400">
              <div className="flex items-start gap-2">
                <MapPin className="w-3.5 h-3.5 text-brand-400 mt-0.5 shrink-0" />
                <span>Lurambi, Kakamega<br />Opposite Bamboo</span>
              </div>
              <div className="flex items-center gap-2">
                <Phone className="w-3.5 h-3.5 text-brand-400" />
                <a href="tel:0704812343" className="hover:text-brand-400 transition-colors font-medium">0704 812 343</a>
              </div>
              <div className="flex items-center gap-2">
                <Mail className="w-3.5 h-3.5 text-brand-400" />
                <a href="mailto:enquiries@naojaventures.com" className="hover:text-brand-400 transition-colors">enquiries@naojaventures.com</a>
              </div>
              <a
                href="https://wa.me/254704812343"
                target="_blank"
                rel="noopener noreferrer"
                className="inline-flex items-center gap-1.5 bg-emerald-600 hover:bg-emerald-700 text-white text-xs font-bold px-3 py-1.5 rounded-lg mt-1 transition-colors"
              >
                💬 Chat on WhatsApp
              </a>
            </div>
          </div>
        </div>
      </div>

      {/* Bottom Bar */}
      <div className="border-t border-gray-800">
        <div className="max-w-7xl mx-auto px-4 py-4 flex flex-col sm:flex-row items-center justify-between text-[11px] text-gray-500 gap-2">
          <p>&copy; {new Date().getFullYear()} Naoja Ventures. All rights reserved.</p>
          <div className="flex items-center gap-3">
            <span className="text-gray-600">Payment:</span>
            <span className="bg-emerald-900/50 text-emerald-400 px-2 py-0.5 rounded font-bold">M-Pesa</span>
            <span className="bg-gray-800 text-gray-400 px-2 py-0.5 rounded font-semibold">Cash</span>
            <span className="text-gray-600 mx-1">|</span>
            <Link to="/admin" className="text-gray-500 hover:text-brand-400 transition-colors">Admin</Link>
          </div>
        </div>
      </div>
    </footer>
  );
}
