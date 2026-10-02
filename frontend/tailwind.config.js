/** @type {import('tailwindcss').Config} */
export default {
  content: [
    "./index.html",
    "./src/**/*.{js,ts,jsx,tsx,html}",
  ],
  theme: {
    extend: {
      colors: {
        // ── Primary brand: rich professional blue ──────────────────────────
        brand: {
          50:  '#EFF6FF',
          100: '#DBEAFE',
          200: '#BFDBFE',
          300: '#93C5FD',
          400: '#60A5FA',
          500: '#3B82F6', // Primary Blue
          600: '#2563EB',
          700: '#1D4ED8',
          800: '#1E40AF',
          900: '#1E3A8A',
        },
        retail: {
          red:    '#E61601',
          orange: '#F97316',
          yellow: '#F59E0B',
          green:  '#059669', // M-Pesa Safaricom Green — keep as-is
          dark:   '#0F172A', // slate-900
          gray:   '#F1F5F9', // slate-100
          border: '#E2E8F0', // slate-200
          blue:   '#1D4ED8',
        },
        surface: {
          DEFAULT: '#F1F5F9', // soft slate background
          card:    '#FFFFFF',
        },
      },
      fontFamily: {
        sans: ['"Plus Jakarta Sans"', 'Inter', 'system-ui', '-apple-system', 'BlinkMacSystemFont', 'Segoe UI', 'Roboto', 'sans-serif'],
        heading: ['"Outfit"', '"Plus Jakarta Sans"', 'sans-serif'],
        display: ['"Outfit"', '"Plus Jakarta Sans"', 'sans-serif'],
      },
      borderRadius: {
        card:    '12px',
        control: '10px',
      },
      boxShadow: {
        hover: '0 8px 20px rgba(16, 24, 40, 0.08)',
        blue:  '0 4px 24px rgba(59, 130, 246, 0.18)',
      },
    },
  },
  plugins: [],
}
