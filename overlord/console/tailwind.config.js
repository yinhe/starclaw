/** @type {import('tailwindcss').Config} */
export default {
  content: ['./index.html', './src/**/*.{js,ts,jsx,tsx}'],
  theme: {
    extend: {
      colors: {
        overlord: {
          50: '#f0f5ff',
          100: '#e0eaff',
          200: '#c7d7fe',
          300: '#a4bbfd',
          400: '#7c96fa',
          500: '#5b6ff4',
          600: '#454de8',
          700: '#393dcd',
          800: '#3035a5',
          900: '#2d3283',
          950: '#1e1f4d',
        },
      },
    },
  },
  plugins: [],
}
