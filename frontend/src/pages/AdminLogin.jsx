import React, { useState } from 'react';
import { useNavigate } from 'react-router-dom';
import { api } from '../lib/api';
import { Lock, Mail, ArrowRight, ShieldCheck } from 'lucide-react';

export default function AdminLogin() {
  const [email, setEmail] = useState('');
  const [password, setPassword] = useState('');
  const [loading, setLoading] = useState(false);
  const [error, setError] = useState('');
  const navigate = useNavigate();

  React.useEffect(() => {
    // Enforce fresh login: clean any prior session tokens when visiting login page
    sessionStorage.removeItem('adminToken');
    sessionStorage.removeItem('adminInfo');
    localStorage.removeItem('adminToken');
    localStorage.removeItem('adminInfo');
  }, []);

  const handleLogin = async (e) => {
    e.preventDefault();
    setLoading(true);
    setError('');

    try {
      const res = await api.admin.login({ email, password });
      if (res.token) {
        // Enforce per-session credentials (sessionStorage cleared upon tab close/exit)
        sessionStorage.setItem('adminToken', res.token);
        sessionStorage.setItem('adminInfo', JSON.stringify(res.admin));
        localStorage.removeItem('adminToken');
        localStorage.removeItem('adminInfo');
        navigate('/admin');
      }
    } catch (err) {
      setError(err.message || 'Failed to login. Please verify email and password.');
    } finally {
      setLoading(false);
    }
  };

  return (
    <div className="min-h-screen bg-slate-50 flex flex-col justify-center py-12 sm:px-6 lg:px-8 font-sans">
      <div className="sm:mx-auto sm:w-full sm:max-w-md text-center">
        <div className="mx-auto w-14 h-14 rounded-2xl bg-gradient-to-tr from-brand-600 to-[#ff6b00] flex items-center justify-center text-white font-display font-black text-2xl shadow-lg mb-4">
          N
        </div>
        <h2 className="text-center text-2xl lg:text-3xl font-display font-extrabold text-gray-950 tracking-tight">
          Naoja Executive Portal
        </h2>
        <p className="mt-2 text-center text-xs text-gray-500 font-medium">
          Sign in to access Naoja Retail Operations & Business Intelligence
        </p>
      </div>

      <div className="mt-8 sm:mx-auto sm:w-full sm:max-w-md">
        <div className="bg-white py-8 px-4 shadow-xl shadow-brand-900/5 sm:rounded-2xl sm:px-10 border border-gray-100">
          
          {error && (
            <div className="mb-6 bg-red-50 border border-red-200 text-red-700 px-4 py-3 rounded-xl text-sm font-medium">
              {error}
            </div>
          )}

          <form className="space-y-6" onSubmit={handleLogin}>
            <div>
              <label className="block text-sm font-semibold text-gray-700">Email address</label>
              <div className="mt-2 relative rounded-md shadow-sm">
                <div className="absolute inset-y-0 left-0 pl-3 flex items-center pointer-events-none">
                  <Mail className="h-5 w-5 text-gray-400" />
                </div>
                <input
                  type="email"
                  required
                  value={email}
                  onChange={(e) => setEmail(e.target.value)}
                  className="block w-full pl-10 h-12 border-gray-300 rounded-xl focus:ring-brand-500 focus:border-brand-500 text-sm border bg-gray-50"
                  placeholder="naojaventures@gmail.com"
                />
              </div>
            </div>

            <div>
              <label className="block text-sm font-semibold text-gray-700">Password</label>
              <div className="mt-2 relative rounded-md shadow-sm">
                <div className="absolute inset-y-0 left-0 pl-3 flex items-center pointer-events-none">
                  <Lock className="h-5 w-5 text-gray-400" />
                </div>
                <input
                  type="password"
                  required
                  value={password}
                  onChange={(e) => setPassword(e.target.value)}
                  className="block w-full pl-10 h-12 border-gray-300 rounded-xl focus:ring-brand-500 focus:border-brand-500 text-sm border bg-gray-50"
                  placeholder="••••••••"
                />
              </div>
            </div>

            <div>
              <button
                type="submit"
                disabled={loading}
                className="w-full flex justify-center items-center gap-2 py-3.5 px-4 border border-transparent rounded-xl shadow-sm text-sm font-bold text-white bg-brand-600 hover:bg-brand-700 focus:outline-none focus:ring-2 focus:ring-offset-2 focus:ring-brand-500 disabled:opacity-70 transition-colors"
              >
                {loading ? 'Authenticating...' : 'Sign In'}
                {!loading && <ArrowRight className="w-4 h-4" />}
              </button>
            </div>
          </form>
        </div>
        
        <p className="mt-6 text-center text-xs text-gray-500 font-medium">
          Protected by AES-256 Encryption & JWT
        </p>
      </div>
    </div>
  );
}
