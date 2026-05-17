import type { Config } from 'tailwindcss'

export default {
  content: ['./index.html', './src/**/*.{ts,tsx}'],
  theme: {
    extend: {
      colors: {
        // Backgrounds
        base:    '#080808',
        surface: '#121212',
        raised:  '#1A1A1A',
        border:  '#2A2A2A',
        // Greens
        forest:  '#1E4620',
        pine:    '#2D6A31',
        fern:    '#4A9E50',
        lime:    '#7CC96E',
        mint:    '#A8E6A0',
        // Text
        muted:   '#666666',
        dim:     '#999999',
        primary: '#E8E8E8',
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
