/**
 * Dynamic brand loading for white-label Overlord instances.
 * Fetches brand config from the API and applies it to the DOM (CSS variables, favicon, title, etc.)
 */

export interface BrandConfig {
  brand_name: string
  logo_url: string
  favicon_url: string
  primary_color: string
  secondary_color: string
  bg_color: string
  accent_color: string
  domain: string
  login_title: string
  login_subtitle: string
  copyright_text: string
  icp_number: string
  support_email: string
  custom_css: string
  powered_by: boolean
  enabled: boolean
}

const BRAND_CACHE_KEY = 'overlord_brand'
const BRAND_CACHE_TTL = 5 * 60 * 1000 // 5 minutes

let _brand: BrandConfig | null = null

// Default brand (StarClaw)
const DEFAULT_BRAND: BrandConfig = {
  brand_name: 'StarClaw',
  logo_url: '',
  favicon_url: '',
  primary_color: '#6d28d9',
  secondary_color: '#4f46e5',
  bg_color: '#0a0a0a',
  accent_color: '#8b5cf6',
  domain: '',
  login_title: '',
  login_subtitle: '',
  copyright_text: '',
  icp_number: '',
  support_email: '',
  custom_css: '',
  powered_by: true,
  enabled: false,
}

/** Load brand config from API (with localStorage cache) */
export async function loadBrand(): Promise<BrandConfig> {
  // Try cache first
  try {
    const cached = localStorage.getItem(BRAND_CACHE_KEY)
    if (cached) {
      const { data, ts } = JSON.parse(cached)
      if (Date.now() - ts < BRAND_CACHE_TTL) {
        _brand = data
        applyBrand(data)
        // Refresh in background
        fetchBrand().catch(() => {})
        return data
      }
    }
  } catch {}

  // Fetch from API
  return fetchBrand()
}

async function fetchBrand(): Promise<BrandConfig> {
  try {
    const res = await fetch('/brood/brand')
    if (!res.ok) throw new Error('Failed to fetch brand')
    const json = await res.json()
    const brand: BrandConfig = { ...DEFAULT_BRAND, ...json.brand }
    _brand = brand
    localStorage.setItem(BRAND_CACHE_KEY, JSON.stringify({ data: brand, ts: Date.now() }))
    applyBrand(brand)
    return brand
  } catch {
    _brand = DEFAULT_BRAND
    applyBrand(DEFAULT_BRAND)
    return DEFAULT_BRAND
  }
}

/** Get the current brand (synchronous, returns cached or default) */
export function getBrand(): BrandConfig {
  return _brand || DEFAULT_BRAND
}

/** Apply brand config to the DOM */
function applyBrand(brand: BrandConfig) {
  if (!brand.enabled) return

  const root = document.documentElement

  // CSS custom properties
  if (brand.primary_color) root.style.setProperty('--brand-primary', brand.primary_color)
  if (brand.secondary_color) root.style.setProperty('--brand-secondary', brand.secondary_color)
  if (brand.accent_color) root.style.setProperty('--brand-accent', brand.accent_color)
  if (brand.bg_color) root.style.setProperty('--brand-bg', brand.bg_color)

  // Favicon
  if (brand.favicon_url) {
    let link = document.querySelector<HTMLLinkElement>('link[rel="icon"]')
    if (!link) {
      link = document.createElement('link')
      link.rel = 'icon'
      document.head.appendChild(link)
    }
    link.href = brand.favicon_url
  }

  // Page title
  if (brand.brand_name) {
    document.title = brand.brand_name
  }

  // Custom CSS injection
  if (brand.custom_css) {
    let style = document.getElementById('brand-custom-css')
    if (!style) {
      style = document.createElement('style')
      style.id = 'brand-custom-css'
      document.head.appendChild(style)
    }
    style.textContent = brand.custom_css
  }
}

/** Invalidate the brand cache (e.g. after admin updates brand config) */
export function invalidateBrandCache() {
  localStorage.removeItem(BRAND_CACHE_KEY)
  _brand = null
}
