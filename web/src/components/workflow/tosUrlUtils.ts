// TOS URL utilities — parse Volcengine Ark pre-signed URLs to compute freshness
// and hybrid-refresh them via /v1/cdn/resign-tos (cheap, 7d) with a fallback to
// /v1/cdn/launder-tos (expensive, 24h).
//
// Background:
//   Seedream /images/generations returns pre-signed TOS URLs with
//   X-Tos-Expires=86400 (24h). The Seedream API does NOT expose an expires
//   knob. Two options to beat the 24h wall:
//     1. Re-sign the SAME object path with our own VOLC_TOS_AK/SK, picking a
//        fresh X-Tos-Date + X-Tos-Expires up to 604800s (7 days) — free HMAC,
//        no Seedream call. Requires our AKSK to have GetObject on the bucket.
//     2. Re-generate via Seedream (a full image_strength=0.01 pass) — costs
//        money + latency, gets you another 24h.
//   We try (1) first, fall back to (2) on bucket ACL denial.

import { cdnAPI } from '../../lib/api'

export interface TOSFreshness {
  /** Whether we could parse the URL (TOS-formatted). */
  parsed: boolean
  /** Signed-at UTC ms since epoch (null if unparseable). */
  signedAtMs: number | null
  /** Expiry UTC ms (null if unparseable). */
  expiresAtMs: number | null
  /** Seconds remaining (negative if already expired). */
  remainingSec: number
  /** True when parsed AND remainingSec > 0. */
  valid: boolean
  /** True when parsed AND remainingSec <= refreshThresholdSec. */
  staleOrExpired: boolean
}

// Refresh when less than 12h left. We now issue 7-day URLs via /v1/cdn/resign-tos
// so we have plenty of head-room; 12h keeps us safely clear of the old 24h
// wall in case resign is unavailable (env missing / bucket ACL denies).
const REFRESH_THRESHOLD_SEC = 60 * 60 * 12

// Parse e.g. "20260422T112324Z" → UTC ms.
function parseTosDate(d: string): number | null {
  const m = /^(\d{4})(\d{2})(\d{2})T(\d{2})(\d{2})(\d{2})Z$/.exec(d)
  if (!m) return null
  const [, y, mo, da, h, mi, s] = m
  return Date.UTC(+y, +mo - 1, +da, +h, +mi, +s)
}

export function parseTOSFreshness(url: string, nowMs = Date.now()): TOSFreshness {
  const empty: TOSFreshness = {
    parsed: false, signedAtMs: null, expiresAtMs: null,
    remainingSec: 0, valid: false, staleOrExpired: false,
  }
  if (!url) return empty
  // Only deal with Ark TOS signed URLs. Plain CDN / local URLs don't have the
  // signature params and are assumed to be permanent (no refresh needed).
  if (!url.includes('X-Tos-Date=') || !url.includes('X-Tos-Expires=')) return empty

  const dateMatch = /X-Tos-Date=([^&]+)/.exec(url)
  const expMatch = /X-Tos-Expires=(\d+)/.exec(url)
  if (!dateMatch || !expMatch) return empty

  const signedAtMs = parseTosDate(decodeURIComponent(dateMatch[1]))
  if (signedAtMs == null) return empty
  const ttlSec = parseInt(expMatch[1], 10)
  if (!Number.isFinite(ttlSec) || ttlSec <= 0) return empty

  const expiresAtMs = signedAtMs + ttlSec * 1000
  const remainingSec = Math.floor((expiresAtMs - nowMs) / 1000)
  return {
    parsed: true,
    signedAtMs,
    expiresAtMs,
    remainingSec,
    valid: remainingSec > 0,
    staleOrExpired: remainingSec <= REFRESH_THRESHOLD_SEC,
  }
}

/** Human-readable freshness label. */
export function freshnessLabel(f: TOSFreshness): { text: string; tone: 'ok' | 'warn' | 'dead' | 'na' } {
  if (!f.parsed) return { text: '非 TOS URL（永久）', tone: 'na' }
  if (f.remainingSec <= 0) {
    const ago = formatDuration(-f.remainingSec)
    return { text: `已过期 ${ago}`, tone: 'dead' }
  }
  const left = formatDuration(f.remainingSec)
  if (f.staleOrExpired) return { text: `剩 ${left}（即将过期）`, tone: 'warn' }
  return { text: `有效 ${left}`, tone: 'ok' }
}

function formatDuration(sec: number): string {
  if (sec < 60) return `${sec}s`
  if (sec < 3600) return `${Math.floor(sec / 60)}m`
  if (sec < 86400) {
    const h = Math.floor(sec / 3600)
    const m = Math.floor((sec % 3600) / 60)
    return m > 0 ? `${h}h ${m}m` : `${h}h`
  }
  const d = Math.floor(sec / 86400)
  const h = Math.floor((sec % 86400) / 3600)
  return h > 0 ? `${d}d ${h}h` : `${d}d`
}

export interface RefreshResult {
  tosUrl: string
  source: 'resign' | 'promote' | 'launder'
  expiresSec?: number
}

/**
 * Hybrid refresh a TOS URL: try /v1/cdn/resign-tos first (cheap, 7d), fall
 * back to /v1/cdn/launder-tos (expensive, 24h). If `oldTOSUrl` is empty or
 * not a TOS URL, skips straight to laundering.
 *
 * @param oldTOSUrl existing (possibly expired) Ark TOS URL, may be empty
 * @param fallbackSource CDN URL or local /v1/projects/... path used when
 *                       resign fails or oldTOSUrl is absent
 */
export async function refreshTOS(oldTOSUrl: string, fallbackSource: string): Promise<RefreshResult> {
  // Path A: resign existing Ark URL if we have one
  if (oldTOSUrl && oldTOSUrl.includes('X-Tos-Algorithm=')) {
    try {
      const { data } = await cdnAPI.resignTOS(oldTOSUrl)
      if (data.tos_url) {
        return { tosUrl: data.tos_url, source: 'resign', expiresSec: data.expires_sec }
      }
    } catch (e) {
      // Log but don't throw — we'll fall back.
      // Typical reasons: 501 env not set, 502 HEAD denied, 400 malformed.
      const err = e as { response?: { status?: number; data?: { error?: string } } }
      console.info(
        `[refreshTOS] resign failed (HTTP ${err.response?.status ?? '?'}: ${err.response?.data?.error ?? 'unknown'}), falling back to promote`,
      )
    }
  }
  const promoteSource = fallbackSource || oldTOSUrl
  if (promoteSource) {
    try {
      const { data } = await cdnAPI.promoteTOS(promoteSource, {
        class: 'derived',
        asset_kind: 'character_sheet',
        variant: 'workflow',
      })
      if (data.tos_url) {
        return { tosUrl: data.tos_url, source: 'promote', expiresSec: data.expires_sec }
      }
    } catch (e) {
      const err = e as { response?: { status?: number; data?: { error?: string } } }
      console.info(
        `[refreshTOS] promote failed (HTTP ${err.response?.status ?? '?'}: ${err.response?.data?.error ?? 'unknown'}), falling back to launder`,
      )
    }
  }
  const launderSource = fallbackSource || oldTOSUrl
  if (!launderSource) {
    throw new Error('No source for laundering (need local/CDN URL or a still-valid TOS URL)')
  }
  const { data } = await cdnAPI.launderTOS(launderSource)
  if (!data.tos_url) throw new Error('Launder returned empty tos_url')
  return { tosUrl: data.tos_url, source: 'launder' }
}
