import { useState, useEffect, useCallback } from 'react'

interface Instance {
  name: string; version: string; status: string; location: string;
  ports: number[]; health: string; health_url: string;
}
interface Platform {
  os: string; arch: string; hostname: string; spore_home: string; version: string;
}
interface QueenStatus {
  bound: boolean; queen_url?: string; user_id?: string; nickname?: string; node_id?: string; bound_at?: string;
}

const api = (path: string, opts?: RequestInit) => fetch(`/api${path}`, opts).then(r => r.json())

function App() {
  const [instances, setInstances] = useState<Instance[]>([])
  const [platform, setPlatform] = useState<Platform | null>(null)
  const [queen, setQueen] = useState<QueenStatus | null>(null)
  const [showBind, setShowBind] = useState(false)
  const [bindEmail, setBindEmail] = useState('')
  const [bindPass, setBindPass] = useState('')
  const [bindLoading, setBindLoading] = useState(false)
  const [bindError, setBindError] = useState('')
  const [update, setUpdate] = useState<any>(null)

  const load = useCallback(async () => {
    const [inst, plat, q] = await Promise.all([
      api('/instances').catch(() => []),
      api('/platform').catch(() => null),
      api('/queen/status').catch(() => null),
    ])
    setInstances(Array.isArray(inst) ? inst : [])
    setPlatform(plat)
    setQueen(q)
  }, [])

  useEffect(() => { load(); const t = setInterval(load, 5000); return () => clearInterval(t) }, [load])

  const action = async (name: string, act: string) => {
    await fetch(`/api/instances/${name}/${act}`, { method: 'POST' })
    setTimeout(load, 1000)
  }

  const checkUpdate = async () => {
    const res = await api('/update/check').catch(() => null)
    setUpdate(res)
  }

  const handleBind = async () => {
    setBindLoading(true); setBindError('')
    try {
      const res = await api('/queen/bind', {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({ email: bindEmail, password: bindPass }),
      })
      if (res.error) { setBindError(res.error) } else { setShowBind(false); load() }
    } catch (e: any) { setBindError(e.message) }
    setBindLoading(false)
  }

  const handleUnbind = async () => {
    await fetch('/api/queen/bind', { method: 'DELETE' })
    load()
  }

  const statusBadge = (s: string) => {
    const m: Record<string, string> = { running: 'badge-green', stopped: 'badge-gray', error: 'badge-red' }
    return <span className={`badge ${m[s] || 'badge-yellow'}`}>{s}</span>
  }
  const healthBadge = (h: string) => {
    if (!h) return null
    const m: Record<string, string> = { healthy: 'badge-green', unhealthy: 'badge-red', unknown: 'badge-yellow' }
    return <span className={`badge ${m[h] || 'badge-gray'}`}>{h}</span>
  }

  return (
    <>
      {/* Header */}
      <div className="flex items-center justify-between mb-2" style={{ marginBottom: 20 }}>
        <div className="flex items-center gap-2">
          <span style={{ fontSize: 28 }}>🦠</span>
          <div>
            <h1 style={{ margin: 0 }}>Spore Desktop</h1>
            <span className="text-xs text-muted">{platform?.os} {platform?.arch} · v{platform?.version}</span>
          </div>
        </div>
        <div className="flex gap-2">
          <button className="btn btn-outline btn-sm" onClick={checkUpdate}>检查更新</button>
          <button className="btn btn-outline btn-sm" onClick={load}>刷新</button>
        </div>
      </div>

      {/* Update Banner */}
      {update?.update_available && (
        <div className="card" style={{ borderColor: 'var(--warning)', background: 'rgba(245,158,11,0.05)' }}>
          <span style={{ color: 'var(--warning)', fontWeight: 600 }}>🆕 {update.message}</span>
        </div>
      )}

      {/* Queen Binding */}
      <div className="card">
        <div className="flex items-center justify-between">
          <h2 style={{ margin: 0 }}>👑 Queen 账号</h2>
          {queen?.bound ? (
            <div className="flex items-center gap-2">
              <span className="badge badge-cyan">{queen.nickname} ({queen.node_id?.slice(0, 12)}...)</span>
              <button className="btn btn-outline btn-sm" onClick={handleUnbind}>解绑</button>
            </div>
          ) : (
            <button className="btn btn-primary btn-sm" onClick={() => setShowBind(!showBind)}>绑定账号</button>
          )}
        </div>
        {!queen?.bound && <p className="text-xs text-muted mt-2">绑定 Queen 账号后可同步成长数据、参与竞技场、使用云船队</p>}

        {showBind && (
          <div className="mt-3" style={{ maxWidth: 360 }}>
            <input className="input mb-2" placeholder="邮箱" value={bindEmail} onChange={e => setBindEmail(e.target.value)} />
            <input className="input mb-2" type="password" placeholder="密码" value={bindPass} onChange={e => setBindPass(e.target.value)} />
            {bindError && <p className="text-xs" style={{ color: 'var(--error)', marginBottom: 8 }}>{bindError}</p>}
            <button className="btn btn-primary" onClick={handleBind} disabled={bindLoading}>
              {bindLoading ? '绑定中...' : '登录并绑定'}
            </button>
          </div>
        )}
      </div>

      {/* Instances */}
      <h2 style={{ marginTop: 20 }}>📦 实例管理</h2>
      {instances.length === 0 ? (
        <div className="card" style={{ textAlign: 'center', padding: 40 }}>
          <p className="text-muted">暂无已安装的实例</p>
          <p className="text-xs text-muted mt-2">使用 Setup 安装器安装 Claw</p>
        </div>
      ) : (
        instances.map(inst => (
          <div className="card" key={inst.name}>
            <div className="flex items-center justify-between mb-2">
              <div className="flex items-center gap-2">
                <span style={{ fontSize: 20 }}>🦞</span>
                <div>
                  <h3 style={{ margin: 0 }}>{inst.name} <span className="text-xs text-muted">v{inst.version}</span></h3>
                  <span className="text-xs text-muted">{inst.location}</span>
                </div>
              </div>
              <div className="flex items-center gap-2">
                {healthBadge(inst.health)}
                {statusBadge(inst.status)}
              </div>
            </div>

            {inst.ports?.length > 0 && (
              <p className="text-xs text-muted mb-2">
                端口: {inst.ports.map(p => (
                  <a key={p} href={`http://localhost:${p}`} target="_blank" rel="noreferrer"
                    style={{ color: 'var(--cyan)', marginRight: 8 }}>:{p}</a>
                ))}
              </p>
            )}

            <div className="flex gap-2 mt-2">
              {inst.status === 'stopped' && <button className="btn btn-success btn-sm" onClick={() => action(inst.name, 'start')}>▶ 启动</button>}
              {inst.status === 'running' && (
                <>
                  <button className="btn btn-outline btn-sm" onClick={() => action(inst.name, 'stop')}>⏹ 停止</button>
                  <button className="btn btn-outline btn-sm" onClick={() => action(inst.name, 'restart')}>🔄 重启</button>
                  {inst.ports?.[0] && (
                    <a className="btn btn-outline btn-sm" href={`http://localhost:${inst.ports[0]}`} target="_blank" rel="noreferrer">🌐 打开</a>
                  )}
                </>
              )}
            </div>
          </div>
        ))
      )}

      {/* Platform */}
      {platform && (
        <div className="card mt-3" style={{ marginTop: 20 }}>
          <h2 style={{ margin: '0 0 8px' }}>🖥 系统信息</h2>
          <div className="row">
            <div className="col"><span className="text-xs text-muted">系统</span><br/><span className="text-sm">{platform.os} / {platform.arch}</span></div>
            <div className="col"><span className="text-xs text-muted">主机名</span><br/><span className="text-sm">{platform.hostname}</span></div>
            <div className="col"><span className="text-xs text-muted">Spore Home</span><br/><span className="text-sm" style={{ fontFamily: 'var(--mono)', fontSize: 11 }}>{platform.spore_home}</span></div>
          </div>
        </div>
      )}
    </>
  )
}

export default App
