import type {
  StarClawConfig,
  ChatCompletionRequest,
  ChatCompletionResponse,
  ChatCompletionChunk,
  Model,
  AgentListResponse,
} from './types'

/**
 * StarClaw JavaScript SDK client.
 *
 * @example
 * ```ts
 * const client = new StarClawClient({
 *   endpoint: 'https://overlord.company.com',
 *   apiKey: 'sk-xxx',
 * })
 * const resp = await client.chat({ model: 'deepseek-chat', messages: [{ role: 'user', content: 'Hello' }] })
 * console.log(resp.choices[0].message.content)
 * ```
 */
export class StarClawClient {
  private endpoint: string
  private apiKey: string
  private timeout: number

  constructor(config: StarClawConfig) {
    this.endpoint = config.endpoint.replace(/\/+$/, '')
    this.apiKey = config.apiKey
    this.timeout = config.timeout ?? 30000
  }

  private headers(): Record<string, string> {
    return {
      'Content-Type': 'application/json',
      Authorization: `Bearer ${this.apiKey}`,
    }
  }

  private async request<T>(method: string, path: string, body?: unknown): Promise<T> {
    const controller = new AbortController()
    const timer = setTimeout(() => controller.abort(), this.timeout)

    try {
      const res = await fetch(`${this.endpoint}${path}`, {
        method,
        headers: this.headers(),
        body: body ? JSON.stringify(body) : undefined,
        signal: controller.signal,
      })

      if (!res.ok) {
        const text = await res.text().catch(() => '')
        throw new Error(`StarClaw API error ${res.status}: ${text}`)
      }

      return (await res.json()) as T
    } finally {
      clearTimeout(timer)
    }
  }

  // ── Chat ──

  /** Create a chat completion (non-streaming) */
  async chat(req: ChatCompletionRequest): Promise<ChatCompletionResponse> {
    return this.request<ChatCompletionResponse>('POST', '/v1/chat/completions', {
      ...req,
      stream: false,
    })
  }

  /**
   * Create a streaming chat completion.
   * Returns an async iterator of chunks.
   *
   * @example
   * ```ts
   * for await (const chunk of client.chatStream({ model: 'deepseek-chat', messages, stream: true })) {
   *   process.stdout.write(chunk.choices[0]?.delta?.content ?? '')
   * }
   * ```
   */
  async *chatStream(req: ChatCompletionRequest): AsyncGenerator<ChatCompletionChunk> {
    const controller = new AbortController()
    const timer = setTimeout(() => controller.abort(), this.timeout * 10) // longer for streaming

    try {
      const res = await fetch(`${this.endpoint}/v1/chat/completions`, {
        method: 'POST',
        headers: { ...this.headers(), Accept: 'text/event-stream' },
        body: JSON.stringify({ ...req, stream: true }),
        signal: controller.signal,
      })

      if (!res.ok) {
        const text = await res.text().catch(() => '')
        throw new Error(`StarClaw API error ${res.status}: ${text}`)
      }

      const reader = res.body?.getReader()
      if (!reader) throw new Error('No response body')

      const decoder = new TextDecoder()
      let buffer = ''

      while (true) {
        const { done, value } = await reader.read()
        if (done) break

        buffer += decoder.decode(value, { stream: true })
        const lines = buffer.split('\n')
        buffer = lines.pop() ?? ''

        for (const line of lines) {
          const trimmed = line.trim()
          if (!trimmed.startsWith('data: ')) continue
          const data = trimmed.slice(6)
          if (data === '[DONE]') return
          try {
            yield JSON.parse(data) as ChatCompletionChunk
          } catch {
            // skip malformed chunks
          }
        }
      }
    } finally {
      clearTimeout(timer)
    }
  }

  // ── Models ──

  /** List available models */
  async listModels(): Promise<Model[]> {
    const res = await this.request<{ data: Model[] }>('GET', '/v1/models')
    return res.data ?? []
  }

  // ── Agents ──

  /** List marketplace agents */
  async listAgents(params?: { category?: string; search?: string; page?: number }): Promise<AgentListResponse> {
    const qs = new URLSearchParams()
    if (params?.category) qs.set('category', params.category)
    if (params?.search) qs.set('search', params.search)
    if (params?.page) qs.set('page', String(params.page))
    const query = qs.toString()
    return this.request<AgentListResponse>('GET', `/marketplace/items${query ? '?' + query : ''}`)
  }

  // ── Health ──

  /** Check API health */
  async health(): Promise<{ status: string; service: string }> {
    return this.request('GET', '/health')
  }
}
