import { create } from 'zustand'

export interface Message {
  id: string
  role: 'user' | 'assistant' | 'system'
  content: string
  attachments?: string
  feedback?: number
  created_at: string
}

export interface Conversation {
  id: string
  agent_id: string
  title: string
  is_pinned?: boolean
  updated_at: string
}

interface ChatState {
  conversations: Conversation[]
  currentConversationId: string | null
  messages: Message[]
  isLoading: boolean
  streamingContent: string
  setConversations: (conversations: Conversation[]) => void
  setCurrentConversation: (id: string | null) => void
  setMessages: (messages: Message[]) => void
  addMessage: (message: Message) => void
  setLoading: (loading: boolean) => void
  setStreamingContent: (content: string) => void
  appendStreamingContent: (chunk: string) => void
}

export const useChatStore = create<ChatState>((set) => ({
  conversations: [],
  currentConversationId: null,
  messages: [],
  isLoading: false,
  streamingContent: '',
  setConversations: (conversations) => set({ conversations }),
  setCurrentConversation: (id) => set({ currentConversationId: id }),
  setMessages: (messages) => set({ messages }),
  addMessage: (message) => set((s) => ({ messages: [...s.messages, message] })),
  setLoading: (isLoading) => set({ isLoading }),
  setStreamingContent: (streamingContent) => set({ streamingContent }),
  appendStreamingContent: (chunk) =>
    set((s) => ({ streamingContent: s.streamingContent + chunk })),
}))
