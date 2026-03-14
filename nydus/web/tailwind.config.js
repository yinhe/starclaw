/** @type {import('tailwindcss').Config} */
export default {
  content: ['./index.html', './src/**/*.{js,ts,jsx,tsx}'],
  theme: {
    extend: {
      colors: {
        nydus: {
          bg: '#0d1117',
          card: '#161b22',
          border: '#30363d',
          text: '#e6edf3',
          muted: '#8b949e',
          dim: '#484f58',
          accent: '#f97316',
          blue: '#58a6ff',
          green: '#3fb950',
        },
      },
    },
  },
  plugins: [require('@tailwindcss/typography')],
}
