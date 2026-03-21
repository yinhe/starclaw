/**
 * Streaming chat example using fetch + SSE.
 *
 * Usage:
 *   npx tsx index.ts "Explain quantum computing"
 */

const endpoint = process.env.STARCLAW_ENDPOINT || 'http://localhost:8080'
const apiKey = process.env.STARCLAW_API_KEY || ''
const prompt = process.argv[2] || 'Hello, what can you do?'

async function main() {
  const res = await fetch(`${endpoint}/v1/chat/completions`, {
    method: 'POST',
    headers: {
      'Content-Type': 'application/json',
      ...(apiKey ? { Authorization: `Bearer ${apiKey}` } : {}),
    },
    body: JSON.stringify({
      model: 'deepseek-chat',
      messages: [{ role: 'user', content: prompt }],
      stream: true,
    }),
  })

  if (!res.ok) {
    console.error(`HTTP ${res.status}: ${await res.text()}`)
    process.exit(1)
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
      if (data === '[DONE]') {
        process.stdout.write('\n')
        return
      }
      try {
        const chunk = JSON.parse(data)
        const content = chunk.choices?.[0]?.delta?.content
        if (content) process.stdout.write(content)
      } catch {}
    }
  }
  process.stdout.write('\n')
}

main().catch(console.error)
