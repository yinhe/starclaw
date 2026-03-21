import { useEffect, useRef, useCallback } from 'react'

export interface TeamMissionUpdate {
  mission_id: string
  instance_id: string
  title: string
  status?: string
  total_steps?: number
  done_steps?: number
  preview_url?: string
}

type EventHandler = (data: TeamMissionUpdate) => void

/**
 * useTeamAgentWS — connects to Overlord's /ws/team-agent endpoint
 * and dispatches real-time mission updates.
 */
export function useTeamAgentWS(teamId: string | undefined, onMissionUpdate: EventHandler) {
  const wsRef = useRef<WebSocket | null>(null)
  const reconnectTimer = useRef<ReturnType<typeof setTimeout>>()
  const handlerRef = useRef(onMissionUpdate)
  handlerRef.current = onMissionUpdate

  const connect = useCallback(() => {
    const proto = window.location.protocol === 'https:' ? 'wss:' : 'ws:'
    const host = window.location.host
    const tid = teamId || 'global'
    const url = `${proto}//${host}/ws/team-agent?team_id=${tid}`

    const ws = new WebSocket(url)
    wsRef.current = ws

    ws.onmessage = (e) => {
      try {
        const msg = JSON.parse(e.data) as { event: string; data: TeamMissionUpdate }
        if (msg.event === 'team_mission_update') {
          handlerRef.current(msg.data)
        }
      } catch { /* ignore malformed */ }
    }

    ws.onclose = () => {
      // Auto-reconnect after 5s
      reconnectTimer.current = setTimeout(connect, 5000)
    }

    ws.onerror = () => ws.close()
  }, [teamId])

  useEffect(() => {
    connect()
    return () => {
      clearTimeout(reconnectTimer.current)
      wsRef.current?.close()
    }
  }, [connect])
}
