/** @type {import('tailwindcss').Config} */
export default {
  content: ['./index.html', './src/**/*.{js,ts,jsx,tsx}'],
  theme: {
    extend: {
      fontFamily: {
        sans: ['Inter', 'system-ui', 'sans-serif'],
      },
      colors: {
        border: 'hsl(var(--border))',
        input: 'hsl(var(--border))',
        ring: 'hsl(var(--ring))',
        background: 'hsl(var(--background))',
        foreground: 'hsl(var(--foreground))',
        primary: {
          DEFAULT: 'hsl(var(--primary))',
          foreground: 'hsl(var(--primary-foreground))',
        },
        destructive: {
          DEFAULT: 'hsl(var(--destructive))',
          foreground: 'hsl(var(--destructive-foreground))',
        },
        muted: {
          DEFAULT: 'hsl(var(--muted))',
          foreground: 'hsl(var(--muted-foreground))',
        },
        card: {
          DEFAULT: 'hsl(var(--card))',
          foreground: 'hsl(var(--card-foreground))',
        },
        glass: {
          white: 'rgba(255, 255, 255, 0.08)',
          light: 'rgba(255, 255, 255, 0.12)',
          medium: 'rgba(255, 255, 255, 0.15)',
          border: 'rgba(255, 255, 255, 0.08)',
          'border-light': 'rgba(255, 255, 255, 0.15)',
        },
        iridescent: {
          blue: '#4facfe',
          purple: '#8b5cf6',
          cyan: '#06b6d4',
          pink: '#ec4899',
        },
      },
      backdropBlur: {
        glass: '20px',
      },
      boxShadow: {
        glass: '0 4px 30px rgba(0, 0, 0, 0.3)',
        'glass-lg': '0 8px 40px rgba(0, 0, 0, 0.4)',
        'glass-glow': '0 0 30px rgba(79, 172, 254, 0.15)',
      },
      animation: {
        shimmer: 'shimmer 3s ease-in-out infinite',
        float: 'float 6s ease-in-out infinite',
        morph: 'morph 8s ease-in-out infinite',
        'pulse-soft': 'pulseSoft 2s ease-in-out infinite',
        gradient: 'gradient 8s ease infinite',
      },
      keyframes: {
        shimmer: {
          '0%, 100%': { opacity: '0.5' },
          '50%': { opacity: '1' },
        },
        float: {
          '0%, 100%': { transform: 'translateY(0px)' },
          '50%': { transform: 'translateY(-20px)' },
        },
        morph: {
          '0%, 100%': { borderRadius: '30% 70% 70% 30% / 30% 30% 70% 70%' },
          '25%': { borderRadius: '58% 42% 75% 25% / 76% 46% 54% 24%' },
          '50%': { borderRadius: '50% 50% 33% 67% / 55% 27% 73% 45%' },
          '75%': { borderRadius: '33% 67% 58% 42% / 63% 68% 32% 37%' },
        },
        pulseSoft: {
          '0%, 100%': { opacity: '0.4' },
          '50%': { opacity: '0.8' },
        },
        gradient: {
          '0%, 100%': { backgroundPosition: '0% 50%' },
          '50%': { backgroundPosition: '100% 50%' },
        },
      },
      backgroundImage: {
        'glass-gradient':
          'linear-gradient(135deg, rgba(79, 172, 254, 0.1), rgba(139, 92, 246, 0.1), rgba(6, 182, 212, 0.1))',
        iridescent:
          'linear-gradient(135deg, #4facfe 0%, #8b5cf6 33%, #06b6d4 66%, #ec4899 100%)',
        'iridescent-subtle':
          'linear-gradient(135deg, rgba(79, 172, 254, 0.15), rgba(139, 92, 246, 0.15), rgba(6, 182, 212, 0.15))',
        'dark-base':
          'linear-gradient(135deg, #0a0a0f 0%, #13132a 50%, #0f0f1a 100%)',
      },
    },
  },
  plugins: [require('tailwindcss-animate')],
}
