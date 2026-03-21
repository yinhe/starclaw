export interface StarClawConfig {
  /** API endpoint, e.g. "https://overlord.company.com" */
  endpoint: string
  /** API key (sk-xxx) */
  apiKey: string
  /** Request timeout in ms (default 30000) */
  timeout?: number
}

export interface ChatMessage {
  role: 'system' | 'user' | 'assistant'
  content: string
}

export interface ChatCompletionRequest {
  model: string
  messages: ChatMessage[]
  stream?: boolean
  temperature?: number
  max_tokens?: number
  top_p?: number
  stop?: string[]
}

export interface ChatChoice {
  index: number
  message: ChatMessage
  finish_reason: string | null
}

export interface ChatUsage {
  prompt_tokens: number
  completion_tokens: number
  total_tokens: number
}

export interface ChatCompletionResponse {
  id: string
  object: string
  created: number
  model: string
  choices: ChatChoice[]
  usage: ChatUsage
}

export interface ChatCompletionChunkDelta {
  role?: string
  content?: string
}

export interface ChatCompletionChunkChoice {
  index: number
  delta: ChatCompletionChunkDelta
  finish_reason: string | null
}

export interface ChatCompletionChunk {
  id: string
  object: string
  created: number
  model: string
  choices: ChatCompletionChunkChoice[]
}

export interface Model {
  id: string
  object: string
  owned_by: string
}

export interface Agent {
  id: string
  name: string
  description: string
  icon: string
  category: string
  status: string
  download_count: number
}

export interface AgentListResponse {
  items: Agent[]
  total: number
}

// ── Team Agent ──

export interface TeamAgentTemplate {
  id: string
  name: string
  category: string
  description: string
  icon: string
  roles: string
  is_official: boolean
  version: string
}

export interface TeamInstance {
  id: string
  template_id: string
  template_name: string
  name: string
  goal: string
  status: string
  energy_budget: number
  energy_used: number
  mission_count: number
  avg_score: number
  created_at: string
}

export interface TeamMission {
  id: string
  instance_id: string
  title: string
  goal: string
  status: string
  total_steps: number
  done_steps: number
  review_score: number
  energy_used: number
  preview_url: string
  created_at: string
  completed_at: string | null
}

export interface CreateTeamInstanceRequest {
  template_id: string
  claw_node_id: string
  name: string
  goal?: string
  energy_budget?: number
}

export interface CreateTeamMissionRequest {
  goal: string
  auto_confirm?: boolean
}
