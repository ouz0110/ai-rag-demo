/** @type {import('tailwindcss').Config} */
export default {
  content: [
    "./index.html",
    "./src/**/*.{vue,js,ts,jsx,tsx}",
  ],
  theme: {
    extend: {
      colors: {
        dark: {
          900: '#090a0f',
          800: '#11131c',
          700: '#1a1d2b',
          600: '#25293c',
        },
      },
    },
  },
  plugins: [],
};
