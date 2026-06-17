/** @type {import('tailwindcss').Config} */
export default {
  content: [
    "./index.html",
    "./src/**/*.{js,ts,jsx,tsx}",
  ],
  theme: {
    extend: {
      colors: {
        brand: {
          50: '#E8F5EE',
          100: '#D1EBDD',
          200: '#BFE8D3',
          300: '#8DD5B5',
          400: '#5BC297',
          500: '#0B6B3A',
          600: '#0A5D33',
          700: '#08512C',
          800: '#063E22',
          900: '#042B18',
        },
        surface: {
          DEFAULT: '#F6F8F7',
          card: '#FFFFFF',
        },
      },
      fontFamily: {
        sans: ['Inter', 'system-ui', '-apple-system', 'Segoe UI', 'Roboto', 'Arial', 'sans-serif'],
        serif: ['Playfair Display', 'Georgia', 'serif'],
      },
      borderRadius: {
        'card': '12px',
        'control': '10px',
      },
      boxShadow: {
        'hover': '0 8px 20px rgba(16, 24, 40, 0.08)',
      },
    },
  },
  plugins: [],
}
