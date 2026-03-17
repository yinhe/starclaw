import { StarClawClient } from './client'
import type { ChatMessage, ChatCompletionChunk } from './types'

/**
 * <starclaw-chat> Web Component
 *
 * Drop-in AI chat widget for any website.
 *
 * @example
 * ```html
 * <script src="https://sdk.starclaw.me/chat-widget.js"></script>
 * <starclaw-chat
 *   endpoint="https://overlord.company.com"
 *   api-key="sk-xxx"
 *   model="deepseek-chat"
 *   title="AI 客服"
 *   placeholder="有什么可以帮您？"
 *   theme="dark"
 * ></starclaw-chat>
 * ```
 */
export class ChatWidget extends HTMLElement {
  private shadow: ShadowRoot
  private client: StarClawClient | null = null
  private messages: ChatMessage[] = []
  private sending = false
  private expanded = false

  // Config from attributes
  private get endpoint() { return this.getAttribute('endpoint') || '' }
  private get apiKey() { return this.getAttribute('api-key') || '' }
  private get model() { return this.getAttribute('model') || 'deepseek-chat' }
  private get chatTitle() { return this.getAttribute('title') || 'AI 助手' }
  private get placeholder() { return this.getAttribute('placeholder') || '输入消息...' }
  private get theme() { return this.getAttribute('theme') || 'dark' }
  private get position() { return this.getAttribute('position') || 'bottom-right' }
  private get systemPrompt() { return this.getAttribute('system-prompt') || '' }

  constructor() {
    super()
    this.shadow = this.attachShadow({ mode: 'open' })
  }

  connectedCallback() {
    if (this.endpoint && this.apiKey) {
      this.client = new StarClawClient({ endpoint: this.endpoint, apiKey: this.apiKey })
    }
    if (this.systemPrompt) {
      this.messages.push({ role: 'system', content: this.systemPrompt })
    }
    this.render()
  }

  private render() {
    const isDark = this.theme === 'dark'
    const bg = isDark ? '#0f172a' : '#ffffff'
    const cardBg = isDark ? '#1e293b' : '#f1f5f9'
    const inputBg = isDark ? '#334155' : '#e2e8f0'
    const textColor = isDark ? '#ffffff' : '#1e293b'
    const mutedColor = isDark ? '#94a3b8' : '#64748b'
    const primary = '#6366f1'

    const posStyle = this.position === 'bottom-left'
      ? 'left: 20px; right: auto;'
      : 'right: 20px; left: auto;'

    this.shadow.innerHTML = `
      <style>
        :host { font-family: -apple-system, BlinkMacSystemFont, 'Segoe UI', sans-serif; }
        .sc-fab {
          position: fixed; bottom: 20px; ${posStyle}
          width: 56px; height: 56px; border-radius: 16px;
          background: ${primary}; color: white; border: none; cursor: pointer;
          display: flex; align-items: center; justify-content: center;
          box-shadow: 0 4px 20px rgba(99,102,241,0.4);
          transition: transform 0.2s, box-shadow 0.2s;
          z-index: 99999;
        }
        .sc-fab:hover { transform: scale(1.08); box-shadow: 0 6px 24px rgba(99,102,241,0.5); }
        .sc-fab svg { width: 24px; height: 24px; fill: currentColor; }
        .sc-panel {
          position: fixed; bottom: 88px; ${posStyle}
          width: 380px; max-height: 560px; border-radius: 16px;
          background: ${bg}; color: ${textColor};
          box-shadow: 0 8px 40px rgba(0,0,0,0.3);
          display: ${this.expanded ? 'flex' : 'none'}; flex-direction: column;
          overflow: hidden; z-index: 99998;
        }
        .sc-header {
          padding: 14px 16px; background: ${cardBg};
          border-bottom: 1px solid ${isDark ? '#334155' : '#e2e8f0'};
          display: flex; align-items: center; gap: 10px;
        }
        .sc-header-title { font-size: 15px; font-weight: 600; flex: 1; }
        .sc-header-close { background: none; border: none; color: ${mutedColor}; cursor: pointer; font-size: 18px; padding: 4px; }
        .sc-messages {
          flex: 1; overflow-y: auto; padding: 12px; min-height: 200px; max-height: 380px;
        }
        .sc-msg { margin-bottom: 12px; display: flex; gap: 8px; }
        .sc-msg-user { justify-content: flex-end; }
        .sc-msg-bubble {
          max-width: 80%; padding: 10px 14px; border-radius: 12px;
          font-size: 13px; line-height: 1.5; white-space: pre-wrap; word-break: break-word;
        }
        .sc-msg-user .sc-msg-bubble { background: ${primary}; color: white; }
        .sc-msg-assistant .sc-msg-bubble { background: ${cardBg}; color: ${textColor}; }
        .sc-input-bar {
          padding: 10px 12px; background: ${cardBg};
          border-top: 1px solid ${isDark ? '#334155' : '#e2e8f0'};
          display: flex; gap: 8px; align-items: center;
        }
        .sc-input {
          flex: 1; padding: 8px 12px; border-radius: 10px; border: none;
          background: ${inputBg}; color: ${textColor}; font-size: 13px; outline: none;
          font-family: inherit;
        }
        .sc-input::placeholder { color: ${mutedColor}; }
        .sc-send {
          width: 36px; height: 36px; border-radius: 10px; border: none;
          background: ${primary}; color: white; cursor: pointer;
          display: flex; align-items: center; justify-content: center;
        }
        .sc-send:disabled { opacity: 0.5; cursor: not-allowed; }
        .sc-send svg { width: 16px; height: 16px; fill: currentColor; }
        .sc-typing { color: ${mutedColor}; font-size: 12px; padding: 4px 0; }
      </style>

      <div class="sc-panel" id="panel">
        <div class="sc-header">
          <div class="sc-header-title">${this.escHtml(this.chatTitle)}</div>
          <button class="sc-header-close" id="close">&times;</button>
        </div>
        <div class="sc-messages" id="messages"></div>
        <div class="sc-input-bar">
          <input class="sc-input" id="input" placeholder="${this.escHtml(this.placeholder)}" />
          <button class="sc-send" id="send">
            <svg viewBox="0 0 24 24"><path d="M2.01 21L23 12 2.01 3 2 10l15 2-15 2z"/></svg>
          </button>
        </div>
      </div>

      <button class="sc-fab" id="fab">
        <svg viewBox="0 0 24 24"><path d="M20 2H4c-1.1 0-2 .9-2 2v18l4-4h14c1.1 0 2-.9 2-2V4c0-1.1-.9-2-2-2zm0 14H6l-2 2V4h16v12z"/></svg>
      </button>
    `

    // Events
    this.shadow.getElementById('fab')!.addEventListener('click', () => this.toggle())
    this.shadow.getElementById('close')!.addEventListener('click', () => this.toggle())
    this.shadow.getElementById('send')!.addEventListener('click', () => this.send())
    this.shadow.getElementById('input')!.addEventListener('keydown', (e: Event) => {
      if ((e as KeyboardEvent).key === 'Enter' && !(e as KeyboardEvent).shiftKey) {
        e.preventDefault()
        this.send()
      }
    })
  }

  private toggle() {
    this.expanded = !this.expanded
    const panel = this.shadow.getElementById('panel')
    if (panel) panel.style.display = this.expanded ? 'flex' : 'none'
  }

  private async send() {
    if (!this.client || this.sending) return
    const input = this.shadow.getElementById('input') as HTMLInputElement
    const text = input.value.trim()
    if (!text) return

    input.value = ''
    this.messages.push({ role: 'user', content: text })
    this.renderMessages()
    this.sending = true
    this.updateSendButton()

    let assistantContent = ''
    this.messages.push({ role: 'assistant', content: '' })

    try {
      const stream = this.client.chatStream({
        model: this.model,
        messages: this.messages.filter(m => m.role !== 'assistant' || m.content),
        stream: true,
      })

      for await (const chunk of stream) {
        const delta = chunk.choices?.[0]?.delta?.content
        if (delta) {
          assistantContent += delta
          this.messages[this.messages.length - 1].content = assistantContent
          this.renderMessages()
        }
      }
    } catch (e) {
      this.messages[this.messages.length - 1].content = assistantContent || `错误: ${e}`
      this.renderMessages()
    } finally {
      this.sending = false
      this.updateSendButton()
    }
  }

  private renderMessages() {
    const container = this.shadow.getElementById('messages')
    if (!container) return

    const visible = this.messages.filter(m => m.role !== 'system')
    container.innerHTML = visible.map(m => `
      <div class="sc-msg sc-msg-${m.role}">
        <div class="sc-msg-bubble">${this.escHtml(m.content) || '<span class="sc-typing">思考中...</span>'}</div>
      </div>
    `).join('')

    container.scrollTop = container.scrollHeight
  }

  private updateSendButton() {
    const btn = this.shadow.getElementById('send') as HTMLButtonElement
    if (btn) btn.disabled = this.sending
  }

  private escHtml(s: string): string {
    const div = document.createElement('div')
    div.textContent = s
    return div.innerHTML
  }
}

// Auto-register
if (typeof customElements !== 'undefined' && !customElements.get('starclaw-chat')) {
  customElements.define('starclaw-chat', ChatWidget)
}
