export type Lang = 'zh' | 'en'

export const t = (zh: string, en: string, lang: Lang) => lang === 'zh' ? zh : en

export function getBrowserLang(): Lang {
  const nav = navigator.language || ''
  return nav.startsWith('zh') ? 'zh' : 'en'
}
