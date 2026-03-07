import { useAuthStore } from '../stores/authStore'

type EventHandler = (data: any) => void

class StarClawWS {
  private ws: WebSocket | null = null
  private handlers: Map<string, Set<EventHandler>> = new Map()
  private reconnectTimer: ReturnType<typeof setTimeout> | null = null
  private reconnectDelay = 1000
  private maxReconnectDelay = 30000

  connect() {
    const token = useAuthStore.getState().token
    if (!token) return

    const proto = window.location.protocol === 'https:' ? 'wss:' : 'ws:'
    const url = `${proto}//${window.location.host}/v1/ws?token=${token}`

    try {
      this.ws = new WebSocket(url)

      this.ws.onopen = () => {
        this.reconnectDelay = 1000
        console.log('[ws] connected')
      }

      this.ws.onmessage = (e) => {
        try {
          const msg = JSON.parse(e.data)
          const handlers = this.handlers.get(msg.event)
          if (handlers) {
            handlers.forEach(fn => fn(msg.data))
          }
          // Also fire wildcard handlers
          const all = this.handlers.get('*')
          if (all) {
            all.forEach(fn => fn(msg))
          }
        } catch { /* ignore parse errors */ }
      }

      this.ws.onclose = () => {
        console.log('[ws] disconnected, reconnecting...')
        this.scheduleReconnect()
      }

      this.ws.onerror = () => {
        this.ws?.close()
      }
    } catch {
      this.scheduleReconnect()
    }
  }

  private scheduleReconnect() {
    if (this.reconnectTimer) return
    this.reconnectTimer = setTimeout(() => {
      this.reconnectTimer = null
      this.reconnectDelay = Math.min(this.reconnectDelay * 2, this.maxReconnectDelay)
      this.connect()
    }, this.reconnectDelay)
  }

  disconnect() {
    if (this.reconnectTimer) {
      clearTimeout(this.reconnectTimer)
      this.reconnectTimer = null
    }
    this.ws?.close()
    this.ws = null
  }

  on(event: string, handler: EventHandler) {
    if (!this.handlers.has(event)) {
      this.handlers.set(event, new Set())
    }
    this.handlers.get(event)!.add(handler)
    return () => this.off(event, handler)
  }

  off(event: string, handler: EventHandler) {
    this.handlers.get(event)?.delete(handler)
  }
}

export const starclawWS = new StarClawWS()
