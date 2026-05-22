# Synera — Color Palette

This document defines the official color palette for the Synera brand and landing page.

## Brand Values & Color Intent

| Color  | Role        | Brand Meaning                                |
|--------|-------------|----------------------------------------------|
| Orange | Primary CTA | Creativity, energy, innovation, action        |
| Blue   | Secondary   | Trust, reliability, professionalism, depth    |
| Gray   | Background  | Clarity, neutrality, modern, clean structure  |

---

## Color Tokens

### Orange (Creativity / Action)

| Token              | Hex       | Usage                                         |
|--------------------|-----------|-----------------------------------------------|
| `--color-orange`   | `#F97316` | Primary CTAs, highlights, accent elements     |
| `--color-orange-dark` | `#EA580C` | Hover states, pressed states on orange CTAs   |
| `--color-orange-light` | `#FED7AA` | Soft backgrounds, badge fills, tags          |
| `--color-orange-subtle` | `#FFF7ED` | Section backgrounds with warm tone           |

### Blue (Trust / Professionalism)

| Token              | Hex       | Usage                                         |
|--------------------|-----------|-----------------------------------------------|
| `--color-blue`     | `#2563EB` | Secondary CTAs, links, nav active state       |
| `--color-blue-dark` | `#1D4ED8` | Hover on blue elements                        |
| `--color-blue-navy` | `#1E3A5F` | Hero background, footer, dark sections        |
| `--color-blue-light` | `#DBEAFE` | Soft blue backgrounds, info badges           |
| `--color-blue-subtle` | `#EFF6FF` | Alternate section backgrounds               |

### Gray (Structure / Clarity)

| Token              | Hex       | Usage                                         |
|--------------------|-----------|-----------------------------------------------|
| `--color-gray-100` | `#F8FAFC` | Page background, section alternation          |
| `--color-gray-200` | `#E2E8F0` | Borders, dividers, card outlines              |
| `--color-gray-400` | `#94A3B8` | Placeholder text, secondary icons             |
| `--color-gray-600` | `#475569` | Body text, secondary content                  |
| `--color-gray-900` | `#0F172A` | Headings, primary text                        |

### Neutrals

| Token              | Hex       | Usage                                         |
|--------------------|-----------|-----------------------------------------------|
| `--color-white`    | `#FFFFFF` | Cards, modal backgrounds, text on dark        |
| `--color-black`    | `#000000` | Sparingly for maximum contrast                |

---

## Usage Guidelines

### CTA Hierarchy
1. **Primary CTA** — Orange (`#F97316`) with white text — "Fale pelo WhatsApp", "Solicitar proposta"
2. **Secondary CTA** — Blue (`#2563EB`) with white text — "Saiba mais", "Ver serviços"
3. **Ghost CTA** — Transparent with blue border — tertiary actions

### Text Contrast
- Dark text (`--color-gray-900`) on light backgrounds (min 4.5:1 ratio)
- White text on `--color-blue-navy` and `--color-orange` backgrounds
- Never use gray text below `--color-gray-600` on white

### Section Backgrounds
- Alternate between `--color-white`, `--color-gray-100`, and `--color-blue-subtle`
- Hero: `--color-blue-navy` with white text
- Footer: `--color-gray-900` with light text

---

## Accessibility

All color combinations must meet **WCAG 2.1 AA** minimum (4.5:1 contrast for normal text, 3:1 for large text).
