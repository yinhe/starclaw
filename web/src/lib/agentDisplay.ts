export function formatAgentDisplayName(name?: string | null): string {
  const text = (name || '').trim()
  if (!text) return ''

  return text
    .replace(/\s*Agent\s*$/i, '')
    .replace(/\s*智能体\s*$/u, '')
    .trim()
}
