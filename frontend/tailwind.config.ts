import type { Config } from 'tailwindcss'

export default {
  content: ['./index.html', './src/**/*.{ts,tsx}'],
  theme: {
    extend: {
      colors: {
        // Backgrounds
        base:    '#060C0A',
        surface: '#0B1510',
        raised:  '#111E17',
        border:  '#1A2C24',
        // Greens
        forest:  '#0E3320',
        pine:    '#1D6040',
        fern:    '#3DC47A',
        lime:    '#6EDFA0',
        mint:    '#AAEDC8',
        // Text
        muted:   '#547066',
        dim:     '#7AA893',
        primary: '#E4EDE8',
      },
      animation: {
        'slide-in-right': 'slide-in-right 0.22s ease-out',
        'slide-in-left':  'slide-in-left  0.22s ease-out',
      },
      keyframes: {
        'slide-in-right': {
          from: { transform: 'translateX(100%)' },
          to:   { transform: 'translateX(0)' },
        },
        'slide-in-left': {
          from: { transform: 'translateX(-100%)' },
          to:   { transform: 'translateX(0)' },
        },
      },
      fontFamily: {
        sans: ['Inter', 'system-ui', 'sans-serif'],
        mono: ['JetBrains Mono', 'monospace'],
      },
      screens: {
        xs: '390px',
      },
    },
  },
  plugins: [],
} satisfies Config
