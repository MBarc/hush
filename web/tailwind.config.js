/** @type {import('tailwindcss').Config} */
export default {
  content: ['./index.html', './src/**/*.{js,jsx}'],
  theme: {
    extend: {
      colors: {
        base: '#0D0B12',
        surface: '#151020',
        raised: '#1D1730',
        border: '#2A2140',
        'border-strong': '#3A2F57',
        primary: '#EDE9F8',
        secondary: '#A79BC4',
        muted: '#6E6389',
        accent: '#8F6FFF',
        'accent-hover': '#A78BFF',
        'accent-deep': '#5F3DD6',
        agent: '#35D0BA',
        'agent-deep': '#1E9C8C',
        success: '#3ECF8E',
        warning: '#F2B24E',
        danger: '#F0546C',
      },
      fontFamily: {
        sans: ['Inter', 'system-ui', 'sans-serif'],
        mono: ['"JetBrains Mono"', 'ui-monospace', 'monospace'],
      },
      borderRadius: {
        card: '10px',
        control: '6px',
      },
    },
  },
  plugins: [],
}
