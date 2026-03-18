import express from 'express'
import fs from 'fs'
import path from 'path'
import cors from 'cors'
import dotenv from 'dotenv'
import { Readable } from 'stream' // 使用 ES 模块的导入语法
import { unlink } from 'fs/promises'
import pdfParse from 'pdf-parse/lib/pdf-parse.js'
import mammoth from 'mammoth'
import xlsx from 'xlsx'
import { Server } from 'socket.io'
import http from 'http'
import axios from 'axios'
import { Blob, File } from 'buffer'
import multer from 'multer'
import WebSocket, { WebSocketServer } from 'ws'
import fss from 'fs-extra'
import sharp from 'sharp' // 用于图像处理
import util from 'util'
import {
  redisClient,
  videoQueue,
  openai,
  fal,
  runwayClient,
  limiter,
  upload,
  dirname,
  API_KEY,
  grokClients,
  getAvailableGrokClients,
  getGrokClient,
  getDefaultGrokClient,
  getNextGrokClient,
  getBestGrokClientForModel,
  getSupportedModels,
  getGrokClientsInfo,
  hasAvailableGrokClients,
  GROK_CONFIGS,
} from './config/index.js'
import { initSocket } from './socket.js'

dotenv.config()

const DATA_DIR = process.env.PROXY_DATA_DIR || dirname

const PROXY_API_KEY = process.env.PROXY_API_KEY || API_KEY

const INTERNAL_FETCH_SECRET =
	process.env.PROXY_INTERNAL_FETCH_SECRET || process.env.API_KEY

const app = express()

app.use(express.json({ limit: '20mb' }))
app.use(cors({}))

function internalFetchAuth(req, res, next) {
	const s = req.get('X-INTERNAL-SECRET')
	if (!INTERNAL_FETCH_SECRET || !s || s !== INTERNAL_FETCH_SECRET) {
		return res.status(401).send({ error: 'Unauthorized' })
	}
	next()
}

app.post('/internal/fetch', internalFetchAuth, async (req, res) => {
	try {
		const url = (req.body?.url || '').toString().trim()
		if (!url) {
			return res.status(400).send({ error: 'url is required' })
		}
		if (!/^https?:\/\//i.test(url)) {
			return res.status(400).send({ error: 'invalid url' })
		}
		// Basic allow-list: only fetch common media hosts to avoid SSRF abuse.
		if (!/\bfal\.media\b/i.test(url)) {
			return res.status(403).send({ error: 'host not allowed' })
		}

		const upstream = await axios.get(url, {
			responseType: 'stream',
			timeout: 10 * 60 * 1000,
			maxRedirects: 5,
			headers: {
				Accept: '*/*',
			},
		})

		const ct = upstream.headers['content-type']
		if (ct) res.setHeader('Content-Type', ct)
		const cl = upstream.headers['content-length']
		if (cl) res.setHeader('Content-Length', cl)
		res.status(200)
		upstream.data.pipe(res)
	} catch (e) {
		const status = e?.response?.status
		if (status) {
			return res.status(status).send({ error: 'upstream error', status })
		}
		return res.status(500).send({ error: 'internal fetch failed' })
	}
})

function isPrivateUrl(rawUrl) {
	try {
		const u = new URL(rawUrl)
		const host = (u.hostname || '').toLowerCase()
		if (host === 'localhost' || host === '127.0.0.1') return true
		if (host.startsWith('192.168.')) return true
		if (host.startsWith('10.')) return true
		const m = host.match(/^172\.(\d+)\./)
		if (m) {
			const n = Number(m[1])
			if (n >= 16 && n <= 31) return true
		}
		return false
	} catch (_) {
		return true
	}
}

function isFalHostedUrl(rawUrl) {
	try {
		const u = new URL(rawUrl)
		const host = (u.hostname || '').toLowerCase()
		return (
			host.endsWith('fal.media') ||
			host.endsWith('fal.run') ||
			host.includes('fal-ai')
		)
	} catch (_) {
		return false
	}
}

async function ensureFalAccessibleFileUrl(rawUrl) {
	const url = (rawUrl || '').toString().trim()
	if (!url) return url
	if (/^https?:\/\//i.test(url) && isFalHostedUrl(url)) return url

	try {
		const resp = await axios.get(url, {
			responseType: 'arraybuffer',
			timeout: 60 * 1000,
			maxRedirects: 5,
			headers: {
				Accept: '*/*',
			},
		})

		const ct = (
			resp.headers?.['content-type'] || 'application/octet-stream'
		).toString()
		const blob = new Blob([resp.data], { type: ct })
		const uploadedUrl = await fal.storage.upload(blob)
		return uploadedUrl
	} catch (e) {
		const err = new Error('Failed to fetch/upload file for Fal')
		err.status = 400
		err.body = {
			detail: [
				{
					loc: ['url'],
					msg:
						'URL 无法被代理服务器访问或上传到 Fal。请提供公网可访问的 https 链接，或先调用 /fal/storage/upload 上传图片获取 fal.media 链接后再提交。',
					type: 'file_download_error',
					input: url,
					reason: e?.message,
				},
			],
		}
		throw err
	}
}

const falStorageUpload = multer({
	storage: multer.memoryStorage(),
	limits: { fileSize: 12 * 1024 * 1024 },
})

app.post(
	'/fal/storage/upload',
	apiKeyValidation,
	falStorageUpload.single('file'),
	async (req, res) => {
		try {
			if (!req.file) {
				return res.status(400).json({ error: 'file is required' })
			}
			const ct = (req.file.mimetype || 'application/octet-stream').toString()
			const name = (req.file.originalname || `upload_${Date.now()}`).toString()
			const f = new File([req.file.buffer], name, { type: ct })
			const url = await fal.storage.upload(f)
			return res.status(200).json({ url })
		} catch (e) {
			console.error('Fal storage upload failed:', e)
			return res.status(500).json({ error: 'fal storage upload failed' })
		}
	}
)

// 添加调试日志（**确保不在生产环境中输出敏感信息**）
console.log(`Loaded OpenAI API Key: ${process.env.API_KEY ? '✔️' : '❌'}`)
console.log(`Loaded Fal AI Key: ${process.env.FAL_KEY ? '✔️' : '❌'}`)
console.log(
	`Loaded RunwayML API Secret: ${process.env.RUNWAYML_API_SECRET ? '✔️' : '❌'}`
)

// Grok API 调试日志
console.log('\n=== Grok API 配置状态 ===')
GROK_CONFIGS.forEach((config, index) => {
	console.log(`Grok API ${config.name}: ${config.enabled ? '✔️' : '❌'}`)
})
const availableGrokClients = getAvailableGrokClients()
console.log(`可用的 Grok API 数量: ${availableGrokClients.length}`)
if (availableGrokClients.length > 0) {
	console.log(`可用的 Grok API: ${availableGrokClients.join(', ')}`)
} else {
	console.log('⚠️  没有配置可用的 Grok API')
}
console.log('========================\n')

// 创建 HTTP 服务器
const server = http.createServer(app)
const io = initSocket(server)

const realtimeWss = new WebSocketServer({ noServer: true })

server.on('upgrade', (req, socket, head) => {
	try {
		const u = new URL(req.url || '/', `http://${req.headers.host || 'localhost'}`)
		if (u.pathname !== '/realtime/ws') return

		const userApiKey = (req.headers['x-api-key'] || '').toString()
		if (!userApiKey || userApiKey !== PROXY_API_KEY) {
			socket.write('HTTP/1.1 401 Unauthorized\r\n\r\n')
			socket.destroy()
			return
		}

		realtimeWss.handleUpgrade(req, socket, head, (ws) => {
			realtimeWss.emit('connection', ws, req)
		})
	} catch (_) {
		try {
			socket.destroy()
		} catch (_) {}
	}
})

realtimeWss.on('connection', (clientWs, req) => {
	const u = new URL(req.url || '/', `http://${req.headers.host || 'localhost'}`)
	const model = (u.searchParams.get('model') || 'gpt-realtime').toString()
	const upstreamUrl = `wss://api.openai.com/v1/realtime?model=${encodeURIComponent(model)}`

	const apiKey = process.env.OPENAI_API_KEY || process.env.API_KEY
	if (!apiKey) {
		try {
			clientWs.send(
				JSON.stringify({ type: 'error', message: 'missing OPENAI_API_KEY' })
			)
			clientWs.close(1011, 'missing OPENAI_API_KEY')
		} catch (_) {}
		return
	}

	let closed = false
	const pending = []
	const upstream = new WebSocket(upstreamUrl, ['realtime'], {
		headers: {
			Authorization: `Bearer ${apiKey}`,
		},
	})

	function closeBoth(code, reason) {
		if (closed) return
		closed = true
		try {
			clientWs.close(code || 1000, reason)
		} catch (_) {}
		try {
			upstream.close(code || 1000, reason)
		} catch (_) {}
	}

	clientWs.on('message', (data, isBinary) => {
		if (closed) return
		// 调试日志：记录客户端发送的消息
		if (!isBinary) {
			try {
				const msg = JSON.parse(data.toString())
				if (msg.type === 'session.update') {
					console.log('[Realtime] 📤 Client session.update:', JSON.stringify(msg, null, 2).substring(0, 2000))
					if (msg.session?.tools) {
						console.log('[Realtime] 🔧 Tools count:', msg.session.tools.length)
					}
				} else if (msg.type?.includes('function')) {
					console.log('[Realtime] 📤 Client function message:', msg.type)
				}
			} catch (_) {}
		}
		if (upstream.readyState === WebSocket.OPEN) {
			try {
				upstream.send(data, { binary: Boolean(isBinary) })
			} catch (_) {}
			return
		}
		if (pending.length < 64) {
			pending.push([data, isBinary])
		}
	})

	clientWs.on('close', () => {
		closeBoth(1000, 'client closed')
	})
	clientWs.on('error', () => {
		closeBoth(1011, 'client error')
	})

	upstream.on('open', () => {
		while (pending.length > 0 && upstream.readyState === WebSocket.OPEN) {
			const [data, isBinary] = pending.shift()
			try {
				upstream.send(data, { binary: Boolean(isBinary) })
			} catch (_) {}
		}
	})

	upstream.on('message', (data, isBinary) => {
		if (closed) return
		// 调试日志：记录上游返回的消息
		if (!isBinary) {
			try {
				const msg = JSON.parse(data.toString())
				if (msg.type?.includes('function_call') || msg.type?.includes('tool')) {
					console.log('[Realtime] 📥 Upstream function/tool message:', msg.type, JSON.stringify(msg).substring(0, 500))
				} else if (msg.type === 'error') {
					console.log('[Realtime] ❌ Upstream error:', JSON.stringify(msg))
				} else if (msg.type === 'session.updated') {
					console.log('[Realtime] ✅ Session updated, tools:', msg.session?.tools?.length || 0)
				}
			} catch (_) {}
		}
		try {
			clientWs.send(data, { binary: Boolean(isBinary) })
		} catch (_) {}
	})

	upstream.on('close', () => {
		try {
			clientWs.send(
				JSON.stringify({ type: 'error', message: 'upstream closed' })
			)
		} catch (_) {}
		closeBoth(1000, 'upstream closed')
	})
	upstream.on('error', (err) => {
		try {
			clientWs.send(
				JSON.stringify({
					type: 'error',
					message: `upstream error: ${err?.message || 'unknown'}`,
				})
			)
		} catch (_) {}
		closeBoth(1011, 'upstream error')
	})
})

// 将Socket.IO实例设置为Express应用的属性
app.set('io', io)

// **在初始化Socket.IO之后再导入路由**
import imageToImageRoutes from './routes/imageToImageRoutes.js'

// 定义存储目录
const VIDEOS_DIR = path.join(DATA_DIR, 'videos')
const UPLOADS_DIR = path.join(DATA_DIR, 'uploads')
const AUDIO_DIR = path.join(DATA_DIR, 'audio')

// 确保目录存在
fss.ensureDirSync(VIDEOS_DIR)
fss.ensureDirSync(UPLOADS_DIR)
fss.ensureDirSync(AUDIO_DIR)

// 添加静态文件服务
app.use('/videos', express.static(VIDEOS_DIR))
app.use('/uploads', express.static(UPLOADS_DIR))
app.use('/audio', express.static(AUDIO_DIR))

// 对于非/audio路径的请求，使用API密钥验证中间件
app.use((req, res, next) => {
	if (
		req.path.startsWith('/audio') ||
		req.path.startsWith('/uploads') ||
		req.path.startsWith('/videos') ||
		req.path.startsWith('/v1/')
	) {
		next() // 如果是/audio路径的请求，直接放行，不进行API密钥验证
	} else {
		apiKeyValidation(req, res, next) // 其他请求走API密钥验证流程
	}
})

// Veo 3.1 Fast - 队列模式：提交任务
app.post('/fal/veo3.1-fast/submit', apiKeyValidation, async (req, res) => {
	try {
		const built = buildVeo31Input(req.body)
		if (built.error) {
			return res.status(built.error.status).json(built.error.payload)
		}

		console.log('提交 Veo 3.1 Fast 队列任务，prompt:', built.input.prompt)
		const result = await fal.queue.submit('fal-ai/veo3.1/fast', {
			input: built.input,
		})
		res.status(202).json(result)
	} catch (error) {
		const status = error?.status || 500
		const requestId = error?.requestId
		const body = error?.body
		const detail = body?.detail

		console.error(
			'Veo 3.1 Fast Submit 调用失败:',
			util.inspect(
				{
					status,
					requestId,
					detail,
					body,
					message: error?.message,
				},
				{ depth: null, maxArrayLength: null }
			)
		)

		res.status(status).json({
			error: 'Veo 3.1 Fast 提交任务失败',
			details: detail || body || error?.message,
			requestId,
		})
	}
})

// Veo 3.1 Fast - 队列模式：查询状态
app.get('/fal/veo3.1-fast/status/:requestId', apiKeyValidation, async (req, res) => {
	try {
		const requestId = req.params.requestId
		const status = await fal.queue.status('fal-ai/veo3.1/fast', {
			requestId,
			logs: true,
		})
		res.json(status)
	} catch (error) {
		const statusCode = error?.status || 500
		const requestId = error?.requestId
		const body = error?.body
		const detail = body?.detail

		console.error(
			'Veo 3.1 Fast Status 调用失败:',
			util.inspect(
				{
					status: statusCode,
					requestId,
					detail,
					body,
					message: error?.message,
				},
				{ depth: null, maxArrayLength: null }
			)
		)

		res.status(statusCode).json({
			error: 'Veo 3.1 Fast 查询状态失败',
			details: detail || body || error?.message,
			requestId,
		})
	}
})

// Veo 3.1 Fast - 队列模式：获取结果
app.get('/fal/veo3.1-fast/result/:requestId', apiKeyValidation, async (req, res) => {
	try {
		const requestId = req.params.requestId
		const result = await fal.queue.result('fal-ai/veo3.1/fast', {
			requestId,
		})
		res.json(result)
	} catch (error) {
		const statusCode = error?.status || 500
		const requestId = error?.requestId
		const body = error?.body
		const detail = body?.detail

		console.error(
			'Veo 3.1 Fast Result 调用失败:',
			util.inspect(
				{
					status: statusCode,
					requestId,
					detail,
					body,
					message: error?.message,
				},
				{ depth: null, maxArrayLength: null }
			)
		)

		res.status(statusCode).json({
			error: 'Veo 3.1 Fast 获取结果失败',
			details: detail || body || error?.message,
			requestId,
		})
	}
})

function buildVeo31FirstLastFrameInput(body) {
	const {
		prompt,
		duration,
		aspect_ratio,
		resolution,
		negative_prompt,
		generate_audio,
		seed,
		auto_fix,
		first_frame_url,
		last_frame_url,
		...otherParams
	} = body || {}

	if (!prompt) {
		return { error: { status: 400, payload: { error: 'prompt 参数是必需的' } } }
	}
	if (!first_frame_url) {
		return {
			error: {
				status: 400,
				payload: { error: 'first_frame_url 参数是必需的' },
			},
		}
	}
	if (!last_frame_url) {
		return {
			error: {
				status: 400,
				payload: { error: 'last_frame_url 参数是必需的' },
			},
		}
	}

	const normalizedDuration = normalizeVeoDuration(duration)
	if (!normalizedDuration) {
		return {
			error: {
				status: 400,
				payload: {
					error: 'duration 参数不合法',
					permitted: ['4s', '6s', '8s'],
					received: duration,
				},
			},
		}
	}

	const normalizedAspect = normalizeAspectRatioVeo31(aspect_ratio)
	if (!normalizedAspect) {
		return {
			error: {
				status: 400,
				payload: {
					error: 'aspect_ratio 参数不合法',
					permitted: ['16:9', '9:16'],
					received: aspect_ratio,
				},
			},
		}
	}

	const normalizedResolution = normalizeResolutionVeo31(resolution)
	if (!normalizedResolution) {
		return {
			error: {
				status: 400,
				payload: {
					error: 'resolution 参数不合法',
					permitted: ['720p', '1080p'],
					received: resolution,
				},
			},
		}
	}

	return {
		input: {
			prompt,
			first_frame_url,
			last_frame_url,
			duration: normalizedDuration,
			aspect_ratio: normalizedAspect,
			resolution: normalizedResolution,
			negative_prompt,
			generate_audio: generate_audio ?? true,
			seed,
			auto_fix: auto_fix ?? true,
			...otherParams,
		},
	}
}

// Veo 3.1 Fast - 首尾帧生成视频（队列模式）：提交任务
app.post(
	'/fal/veo3.1-fast/first-last-frame-to-video/submit',
	apiKeyValidation,
	async (req, res) => {
		try {
			const built = buildVeo31FirstLastFrameInput(req.body)
			if (built.error) {
				return res.status(built.error.status).json(built.error.payload)
			}

			const fixedFirst = await ensureFalAccessibleFileUrl(
				built.input.first_frame_url
			)
			const fixedLast = await ensureFalAccessibleFileUrl(
				built.input.last_frame_url
			)
			const fixedInput = {
				...built.input,
				first_frame_url: fixedFirst,
				last_frame_url: fixedLast,
			}

			console.log('首尾帧 URL 重写:', {
				original_first: built.input.first_frame_url,
				original_last: built.input.last_frame_url,
				fixed_first: fixedFirst,
				fixed_last: fixedLast,
			})

			console.log(
				'提交 Veo 3.1 Fast 首尾帧队列任务，prompt:',
				fixedInput.prompt
			)
			const result = await fal.queue.submit(
				'fal-ai/veo3.1/fast/first-last-frame-to-video',
				{
					input: fixedInput,
				}
			)
			res.status(202).json({
				...result,
				debug_fixed_first_frame_url: fixedFirst,
				debug_fixed_last_frame_url: fixedLast,
			})
		} catch (error) {
			const status = error?.status || 500
			const requestId = error?.requestId
			const body = error?.body
			const detail = body?.detail

			console.error(
				'Veo 3.1 Fast First-Last Submit 调用失败:',
				util.inspect(
					{
						status,
						requestId,
						detail,
						body,
						message: error?.message,
					},
					{ depth: null, maxArrayLength: null }
				)
			)

			res.status(status).json({
				error: 'Veo 3.1 Fast 首尾帧提交任务失败',
				details: detail || body || error?.message,
				requestId,
			})
		}
	}
)

// Veo 3.1 Fast - 首尾帧生成视频（队列模式）：查询状态
app.get(
	'/fal/veo3.1-fast/first-last-frame-to-video/status/:requestId',
	apiKeyValidation,
	async (req, res) => {
		try {
			const requestId = req.params.requestId
			const status = await fal.queue.status(
				'fal-ai/veo3.1/fast/first-last-frame-to-video',
				{
					requestId,
					logs: true,
				}
			)
			res.json(status)
		} catch (error) {
			const statusCode = error?.status || 500
			const requestId = error?.requestId
			const body = error?.body
			const detail = body?.detail

			console.error(
				'Veo 3.1 Fast First-Last Status 调用失败:',
				util.inspect(
					{
						status: statusCode,
						requestId,
						detail,
						body,
						message: error?.message,
					},
					{ depth: null, maxArrayLength: null }
				)
			)

			res.status(statusCode).json({
				error: 'Veo 3.1 Fast 首尾帧查询状态失败',
				details: detail || body || error?.message,
				requestId,
			})
		}
	}
)

// Veo 3.1 Fast - 首尾帧生成视频（队列模式）：获取结果
app.get(
	'/fal/veo3.1-fast/first-last-frame-to-video/result/:requestId',
	apiKeyValidation,
	async (req, res) => {
		try {
			const requestId = req.params.requestId
			const result = await fal.queue.result(
				'fal-ai/veo3.1/fast/first-last-frame-to-video',
				{
					requestId,
				}
			)
			res.json(result)
		} catch (error) {
			const statusCode = error?.status || 500
			const requestId = error?.requestId
			const body = error?.body
			const detail = body?.detail

			console.error(
				'Veo 3.1 Fast First-Last Result 调用失败:',
				util.inspect(
					{
						status: statusCode,
						requestId,
						detail,
						body,
						message: error?.message,
					},
					{ depth: null, maxArrayLength: null }
				)
			)

			res.status(statusCode).json({
				error: 'Veo 3.1 Fast 首尾帧获取结果失败',
				details: detail || body || error?.message,
				requestId,
			})
		}
	}
)

// API密钥验证中间件
function apiKeyValidation(req, res, next) {
	const userApiKey = req.get('X-API-KEY') // 获取API密钥
	// console.log('Received API Key:', userApiKey) // 输出接收到的API密钥
	// console.log('Expected API Key:', PROXY_API_KEY) // 输出期望的API密钥

	if (userApiKey && userApiKey === PROXY_API_KEY) {
		// console.log('API Key is valid') // 验证通过的日志
		next() // API密钥匹配，继续处理请求
	} else {
		res.status(401).send({ error: 'API Key is invalid or missing' })
	}
}

// === Gateway 中转（供 star-ai.net Gateway 代理海外 API） ===
// 根据 model 前缀自动路由到正确的上游 Provider
const PROXY_INTERNAL_SECRET = process.env.PROXY_INTERNAL_SECRET || ''
const GATEWAY_PROVIDERS = {
	openai:    { url: 'https://api.openai.com/v1',                           key: process.env.OPENAI_API_KEY || process.env.API_KEY },
	grok:      { url: 'https://api.x.ai/v1',                                key: process.env.GROK_API_KEY || process.env.GROK_API_KEY_4 },
	anthropic: { url: 'https://api.anthropic.com/v1',                        key: process.env.ANTHROPIC_API_KEY },
	gemini:    { url: 'https://generativelanguage.googleapis.com/v1beta/openai', key: process.env.GOOGLE_API_KEY },
}

function resolveGatewayProvider(model) {
	if (!model) return null
	if (model.startsWith('gpt-') || model.startsWith('o1') || model.startsWith('o3') || model.startsWith('o4') || model.startsWith('chatgpt-') || model.startsWith('codex-')) return 'openai'
	if (model.startsWith('grok-')) return 'grok'
	if (model.startsWith('claude-')) return 'anthropic'
	if (model.startsWith('gemini-')) return 'gemini'
	return 'openai' // default fallback
}

function gatewayAuth(req, res, next) {
	const xKey = (req.get('X-API-KEY') || '').toString()
	const bearer = (req.get('Authorization') || '').replace(/^Bearer\s+/i, '').trim()
	const key = xKey || bearer
	const validKeys = [PROXY_API_KEY, PROXY_INTERNAL_SECRET].filter(Boolean)
	if (key && validKeys.includes(key)) {
		next()
	} else {
		res.status(401).json({ error: { message: 'API Key is invalid or missing' } })
	}
}

app.post('/v1/chat/completions', gatewayAuth, async (req, res) => {
	try {
		const model = (req.body?.model || '').toString()
		const providerName = resolveGatewayProvider(model)
		console.log(`[gateway] ${model} → ${providerName}`)

		// Grok: 使用 OpenAI SDK client（避免 Cloudflare 拦截）
		if (providerName === 'grok') {
			const grokClient = getBestGrokClientForModel(model) || getDefaultGrokClient()
			if (!grokClient) {
				return res.status(400).json({ error: { message: 'Grok API not configured on proxy' } })
			}
			if (req.body?.stream) {
				const stream = await grokClient.chat.completions.create({ ...req.body, stream: true })
				res.setHeader('Content-Type', 'text/event-stream')
				res.setHeader('Cache-Control', 'no-cache')
				res.setHeader('Connection', 'keep-alive')
				for await (const chunk of stream) {
					res.write(`data: ${JSON.stringify(chunk)}\n\n`)
				}
				res.write('data: [DONE]\n\n')
				res.end()
			} else {
				const result = await grokClient.chat.completions.create(req.body)
				res.json(result)
			}
			return
		}

		// 其他 Provider: 使用 axios 直接转发
		const provider = GATEWAY_PROVIDERS[providerName]
		if (!provider || !provider.key) {
			return res.status(400).json({ error: { message: `provider "${providerName}" not configured on proxy` } })
		}
		const upstreamUrl = `${provider.url}/chat/completions`
		const upstreamRes = await axios({
			method: 'POST',
			url: upstreamUrl,
			headers: {
				'Content-Type': 'application/json',
				'Authorization': `Bearer ${provider.key}`,
			},
			data: req.body,
			responseType: req.body?.stream ? 'stream' : 'json',
			timeout: 300000,
		})
		if (req.body?.stream) {
			res.setHeader('Content-Type', 'text/event-stream')
			res.setHeader('Cache-Control', 'no-cache')
			res.setHeader('Connection', 'keep-alive')
			upstreamRes.data.pipe(res)
		} else {
			res.status(upstreamRes.status).json(upstreamRes.data)
		}
	} catch (e) {
		const status = e.response?.status || e.status || 502
		const data = e.response?.data || { error: { message: e.message } }
		res.status(status).json(data)
	}
})

// 全局应用到所有路由
app.use(limiter)

// 图生图路由
app.use('/fal/image-to-image', imageToImageRoutes)

// === 新增 fal.ai 接口 ===

function normalizeVeoDuration(duration) {
	const permitted = ['4s', '6s', '8s']
	if (duration === undefined || duration === null || duration === '') return '6s'
	if (typeof duration === 'string') {
		const d = duration.trim()
		if (permitted.includes(d)) return d
		const num = Number(d.replace(/s$/i, ''))
		if (!Number.isNaN(num)) {
			if (num <= 4) return '4s'
			if (num <= 6) return '6s'
			return '8s'
		}
		return null
	}
	if (typeof duration === 'number') {
		if (duration <= 4) return '4s'
		if (duration <= 6) return '6s'
		return '8s'
	}
	return null
}

function normalizeAspectRatioVeo31(aspectRatio) {
	const permitted = ['16:9', '9:16', 'auto']
	if (aspectRatio === undefined || aspectRatio === null || aspectRatio === '') return '16:9'
	if (typeof aspectRatio === 'string') {
		const v = aspectRatio.trim()
		if (v === 'auto') return '16:9'
		return permitted.includes(v) ? v : null
	}
	return null
}

function normalizeResolutionVeo31(resolution) {
	const permitted = ['720p', '1080p']
	if (resolution === undefined || resolution === null || resolution === '') return '720p'
	if (typeof resolution === 'string') {
		const v = resolution.trim()
		return permitted.includes(v) ? v : null
	}
	return null
}

function buildVeo31Input(body) {
	const {
		prompt,
		duration,
		aspect_ratio,
		resolution,
		negative_prompt,
		generate_audio,
		seed,
		auto_fix,
		...otherParams
	} = body || {}

	if (!prompt) {
		return { error: { status: 400, payload: { error: 'prompt 参数是必需的' } } }
	}

	const normalizedDuration = normalizeVeoDuration(duration)
	if (!normalizedDuration) {
		return {
			error: {
				status: 400,
				payload: {
					error: 'duration 参数不合法',
					permitted: ['4s', '6s', '8s'],
					received: duration,
				},
			},
		}
	}

	const normalizedAspect = normalizeAspectRatioVeo31(aspect_ratio)
	if (!normalizedAspect) {
		return {
			error: {
				status: 400,
				payload: {
					error: 'aspect_ratio 参数不合法',
					permitted: ['16:9', '9:16'],
					received: aspect_ratio,
				},
			},
		}
	}

	const normalizedResolution = normalizeResolutionVeo31(resolution)
	if (!normalizedResolution) {
		return {
			error: {
				status: 400,
				payload: {
					error: 'resolution 参数不合法',
					permitted: ['720p', '1080p'],
					received: resolution,
				},
			},
		}
	}

	return {
		input: {
			prompt,
			duration: normalizedDuration,
			aspect_ratio: normalizedAspect,
			resolution: normalizedResolution,
			negative_prompt,
			generate_audio: generate_audio ?? true,
			seed,
			auto_fix: auto_fix ?? true,
			...otherParams,
		},
	}
}

// Veo 3 Fast - 文本生成视频（快速版）
app.post('/fal/veo3-fast', apiKeyValidation, async (req, res) => {
	try {
		const { prompt, duration, aspect_ratio = '16:9', ...otherParams } = req.body
		
		if (!prompt) {
			return res.status(400).json({ error: 'prompt 参数是必需的' })
		}

		const normalizedDuration = normalizeVeoDuration(duration)
		if (!normalizedDuration) {
			return res.status(400).json({
				error: "duration 参数不合法",
				permitted: ['4s', '6s', '8s'],
				received: duration,
			})
		}
		
		console.log('调用 Veo 3 Fast API，prompt:', prompt)
		
		const result = await fal.subscribe('fal-ai/veo3/fast', {
			input: {
				prompt,
				duration: normalizedDuration,
				aspect_ratio,
				...otherParams
			},
			logs: true,
			onQueueUpdate: (update) => {
				console.log('Veo 3 Fast 队列状态:', update.status)
			}
		})
		
		console.log('Veo 3 Fast 生成完成')
		res.json(result)
	} catch (error) {
		const status = error?.status || 500
		const requestId = error?.requestId
		const body = error?.body
		const detail = body?.detail

		console.error(
			'Veo 3 Fast API 调用失败:',
			util.inspect(
				{
					status,
					requestId,
					detail,
					body,
					message: error?.message,
				},
				{ depth: null, maxArrayLength: null }
			)
		)

		res.status(status).json({
			error: 'Veo 3 Fast 视频生成失败',
			details: detail || body || error?.message,
			requestId,
		})
	}
})

// Veo 3.1 Fast - 文本生成视频（快速版）
app.post('/fal/veo3.1-fast', apiKeyValidation, async (req, res) => {
	try {
		const built = buildVeo31Input(req.body)
		if (built.error) {
			return res.status(built.error.status).json(built.error.payload)
		}

		console.log('调用 Veo 3.1 Fast API，prompt:', built.input.prompt)

		const result = await fal.subscribe('fal-ai/veo3.1/fast', {
			input: built.input,
			logs: true,
			onQueueUpdate: (update) => {
				console.log('Veo 3.1 Fast 队列状态:', update.status)
			},
		})

		console.log('Veo 3.1 Fast 生成完成')
		res.json(result)
	} catch (error) {
		const status = error?.status || 500
		const requestId = error?.requestId
		const body = error?.body
		const detail = body?.detail

		console.error(
			'Veo 3.1 Fast API 调用失败:',
			util.inspect(
				{
					status,
					requestId,
					detail,
					body,
					message: error?.message,
				},
				{ depth: null, maxArrayLength: null }
			)
		)

		res.status(status).json({
			error: 'Veo 3.1 Fast 视频生成失败',
			details: detail || body || error?.message,
			requestId,
		})
	}
})

// Veo 2 - 图片生成视频
app.post('/fal/veo2-image-to-video', apiKeyValidation, async (req, res) => {
	try {
		const { image_url, prompt, duration = 5, aspect_ratio = '16:9', ...otherParams } = req.body
		
		if (!image_url) {
			return res.status(400).json({ error: 'image_url 参数是必需的' })
		}
		
		console.log('调用 Veo 2 Image-to-Video API，图片:', image_url)
		
		const result = await fal.subscribe('fal-ai/veo2/image-to-video', {
			input: {
				image_url,
				prompt,
				duration,
				aspect_ratio,
				...otherParams
			},
			logs: true,
			onQueueUpdate: (update) => {
				console.log('Veo 2 Image-to-Video 队列状态:', update.status)
			}
		})
		
		console.log('Veo 2 Image-to-Video 生成完成')
		res.json(result)
	} catch (error) {
		console.error('Veo 2 Image-to-Video API 调用失败:', error)
		res.status(500).json({ 
			error: 'Veo 2 图片生成视频失败', 
			details: error.message 
		})
	}
})

// Veo 3 - 文本生成视频（标准版）
app.post('/fal/veo3', apiKeyValidation, async (req, res) => {
	try {
		const { prompt, duration = 5, aspect_ratio = '16:9', ...otherParams } = req.body
		
		if (!prompt) {
			return res.status(400).json({ error: 'prompt 参数是必需的' })
		}
		
		console.log('调用 Veo 3 API，prompt:', prompt)
		
		const result = await fal.subscribe('fal-ai/veo3', {
			input: {
				prompt,
				duration,
				aspect_ratio,
				...otherParams
			},
			logs: true,
			onQueueUpdate: (update) => {
				console.log('Veo 3 队列状态:', update.status)
			}
		})
		
		console.log('Veo 3 生成完成')
		res.json(result)
	} catch (error) {
		console.error('Veo 3 API 调用失败:', error)
		res.status(500).json({ 
			error: 'Veo 3 视频生成失败', 
			details: error.message 
		})
	}
})

// Veo 3 Image-to-Video - 图像生成视频（标准版）
app.post('/fal/veo3-image-to-video', apiKeyValidation, async (req, res) => {
	try {
		const { 
			image_url, 
			prompt, 
			duration = 5, 
			aspect_ratio = '16:9',
			motion_strength = 0.8,
			...otherParams 
		} = req.body
		
		if (!image_url) {
			return res.status(400).json({ error: 'image_url 参数是必需的' })
		}
		
		console.log('调用 Veo 3 Image-to-Video API，图片:', image_url)
		
		const result = await fal.subscribe('fal-ai/veo3/image-to-video', {
			input: {
				image_url,
				prompt,
				duration,
				aspect_ratio,
				motion_strength,
				...otherParams
			},
			logs: true,
			onQueueUpdate: (update) => {
				console.log('Veo 3 Image-to-Video 队列状态:', update.status)
			}
		})
		
		console.log('Veo 3 Image-to-Video 生成完成')
		res.json(result)
	} catch (error) {
		console.error('Veo 3 Image-to-Video API 调用失败:', error)
		res.status(500).json({ 
			error: 'Veo 3 图像生成视频失败', 
			details: error.message 
		})
	}
})

// Veo 3 Fast Image-to-Video - 图像生成视频（快速版）
app.post('/fal/veo3-fast-image-to-video', apiKeyValidation, async (req, res) => {
	try {
		const { 
			image_url, 
			prompt, 
			duration = 5, 
			aspect_ratio = '16:9',
			motion_strength = 0.8,
			...otherParams 
		} = req.body
		
		if (!image_url) {
			return res.status(400).json({ error: 'image_url 参数是必需的' })
		}
		
		console.log('调用 Veo 3 Fast Image-to-Video API，图片:', image_url)
		
		const result = await fal.subscribe('fal-ai/veo3/fast/image-to-video', {
			input: {
				image_url,
				prompt,
				duration,
				aspect_ratio,
				motion_strength,
				...otherParams
			},
			logs: true,
			onQueueUpdate: (update) => {
				console.log('Veo 3 Fast Image-to-Video 队列状态:', update.status)
			}
		})
		
		console.log('Veo 3 Fast Image-to-Video 生成完成')
		res.json(result)
	} catch (error) {
		console.error('Veo 3 Fast Image-to-Video API 调用失败:', error)
		res.status(500).json({ 
			error: 'Veo 3 Fast 图像生成视频失败', 
			details: error.message 
		})
	}
})

// === Kling V3 Pro 视频生成接口 ===

function normalizeKlingV3Duration(duration) {
	// Kling V3 accepts "3" through "15" as string
	const permitted = [3, 4, 5, 6, 7, 8, 9, 10, 11, 12, 13, 14, 15]
	if (duration === undefined || duration === null || duration === '') return '5'
	const num = typeof duration === 'number' ? duration : Number(String(duration).replace(/s$/i, ''))
	if (Number.isNaN(num)) return null
	const clamped = Math.max(3, Math.min(15, Math.round(num)))
	return String(clamped)
}

function normalizeKlingV3AspectRatio(aspectRatio) {
	const permitted = ['16:9', '9:16', '1:1']
	if (!aspectRatio) return '16:9'
	const v = String(aspectRatio).trim()
	return permitted.includes(v) ? v : null
}

function buildKlingV3T2VInput(body) {
	const { prompt, duration, aspect_ratio, generate_audio, negative_prompt, cfg_scale, ...otherParams } = body || {}
	if (!prompt) return { error: { status: 400, payload: { error: 'prompt 参数是必需的' } } }
	const normalizedDuration = normalizeKlingV3Duration(duration)
	if (!normalizedDuration) return { error: { status: 400, payload: { error: 'duration 参数不合法', permitted: '3-15', received: duration } } }
	const normalizedAspect = normalizeKlingV3AspectRatio(aspect_ratio)
	if (!normalizedAspect) return { error: { status: 400, payload: { error: 'aspect_ratio 参数不合法', permitted: ['16:9', '9:16', '1:1'], received: aspect_ratio } } }
	return {
		input: {
			prompt,
			duration: normalizedDuration,
			aspect_ratio: normalizedAspect,
			generate_audio: generate_audio ?? true,
			negative_prompt: negative_prompt || 'blur, distort, and low quality',
			cfg_scale: cfg_scale ?? 0.5,
			...otherParams,
		},
	}
}

async function buildKlingV3I2VInput(body) {
	const { prompt, start_image_url, image_url, duration, generate_audio, negative_prompt, cfg_scale, end_image_url, ...otherParams } = body || {}
	if (!prompt) return { error: { status: 400, payload: { error: 'prompt 参数是必需的' } } }
	const rawImgUrl = start_image_url || image_url
	if (!rawImgUrl) return { error: { status: 400, payload: { error: 'start_image_url 或 image_url 参数是必需的' } } }
	const normalizedDuration = normalizeKlingV3Duration(duration)
	if (!normalizedDuration) return { error: { status: 400, payload: { error: 'duration 参数不合法', permitted: '3-15', received: duration } } }
	let fixedImgUrl
	try { fixedImgUrl = await ensureFalAccessibleFileUrl(rawImgUrl) } catch (e) {
		return { error: { status: e.status || 400, payload: e.body || { error: 'start_image_url 无法访问' } } }
	}
	const input = {
		prompt,
		start_image_url: fixedImgUrl,
		duration: normalizedDuration,
		generate_audio: generate_audio ?? true,
		negative_prompt: negative_prompt || 'blur, distort, and low quality',
		cfg_scale: cfg_scale ?? 0.5,
		...otherParams,
	}
	if (end_image_url) {
		try { input.end_image_url = await ensureFalAccessibleFileUrl(end_image_url) } catch (_) {}
	}
	return { input }
}

// Kling V3 Pro - 文生视频（同步）
app.post('/fal/kling-v3-pro', apiKeyValidation, async (req, res) => {
	try {
		const built = buildKlingV3T2VInput(req.body)
		if (built.error) return res.status(built.error.status).json(built.error.payload)
		console.log('调用 Kling V3 Pro T2V API，prompt:', built.input.prompt)
		const result = await fal.subscribe('fal-ai/kling-video/v3/pro/text-to-video', {
			input: built.input,
			logs: true,
			onQueueUpdate: (update) => { console.log('Kling V3 Pro T2V 队列状态:', update.status) },
		})
		console.log('Kling V3 Pro T2V 生成完成')
		res.json(result)
	} catch (error) {
		const status = error?.status || 500
		console.error('Kling V3 Pro T2V 调用失败:', util.inspect({ status, requestId: error?.requestId, detail: error?.body?.detail, body: error?.body, message: error?.message }, { depth: null, maxArrayLength: null }))
		res.status(status).json({ error: 'Kling V3 Pro 文生视频失败', details: error?.body?.detail || error?.body || error?.message, requestId: error?.requestId })
	}
})

// Kling V3 Pro - 文生视频（队列模式）：提交
app.post('/fal/kling-v3-pro/submit', apiKeyValidation, async (req, res) => {
	try {
		const built = buildKlingV3T2VInput(req.body)
		if (built.error) return res.status(built.error.status).json(built.error.payload)
		console.log('提交 Kling V3 Pro T2V 队列任务，prompt:', built.input.prompt)
		const result = await fal.queue.submit('fal-ai/kling-video/v3/pro/text-to-video', { input: built.input })
		res.status(202).json(result)
	} catch (error) {
		const status = error?.status || 500
		console.error('Kling V3 Pro T2V Submit 失败:', util.inspect({ status, detail: error?.body?.detail, message: error?.message }, { depth: null }))
		res.status(status).json({ error: 'Kling V3 Pro 提交任务失败', details: error?.body?.detail || error?.body || error?.message, requestId: error?.requestId })
	}
})

// Kling V3 Pro - 文生视频（队列模式）：状态
app.get('/fal/kling-v3-pro/status/:requestId', apiKeyValidation, async (req, res) => {
	try {
		const status = await fal.queue.status('fal-ai/kling-video/v3/pro/text-to-video', { requestId: req.params.requestId, logs: true })
		res.json(status)
	} catch (error) {
		const statusCode = error?.status || 500
		console.error('Kling V3 Pro T2V Status 失败:', error?.message)
		res.status(statusCode).json({ error: 'Kling V3 Pro 查询状态失败', details: error?.body?.detail || error?.body || error?.message })
	}
})

// Kling V3 Pro - 文生视频（队列模式）：结果
app.get('/fal/kling-v3-pro/result/:requestId', apiKeyValidation, async (req, res) => {
	try {
		const result = await fal.queue.result('fal-ai/kling-video/v3/pro/text-to-video', { requestId: req.params.requestId })
		res.json(result)
	} catch (error) {
		const statusCode = error?.status || 500
		console.error('Kling V3 Pro T2V Result 失败:', error?.message)
		res.status(statusCode).json({ error: 'Kling V3 Pro 获取结果失败', details: error?.body?.detail || error?.body || error?.message })
	}
})

// Kling V3 Pro - 图生视频（同步）
app.post('/fal/kling-v3-pro-image-to-video', apiKeyValidation, async (req, res) => {
	try {
		const built = await buildKlingV3I2VInput(req.body)
		if (built.error) return res.status(built.error.status).json(built.error.payload)
		console.log('调用 Kling V3 Pro I2V API，prompt:', built.input.prompt)
		const result = await fal.subscribe('fal-ai/kling-video/v3/pro/image-to-video', {
			input: built.input,
			logs: true,
			onQueueUpdate: (update) => { console.log('Kling V3 Pro I2V 队列状态:', update.status) },
		})
		console.log('Kling V3 Pro I2V 生成完成')
		res.json(result)
	} catch (error) {
		const status = error?.status || 500
		console.error('Kling V3 Pro I2V 调用失败:', util.inspect({ status, requestId: error?.requestId, detail: error?.body?.detail, body: error?.body, message: error?.message }, { depth: null, maxArrayLength: null }))
		res.status(status).json({ error: 'Kling V3 Pro 图生视频失败', details: error?.body?.detail || error?.body || error?.message, requestId: error?.requestId })
	}
})

// Kling V3 Pro - 图生视频（队列模式）：提交
app.post('/fal/kling-v3-pro-image-to-video/submit', apiKeyValidation, async (req, res) => {
	try {
		const built = await buildKlingV3I2VInput(req.body)
		if (built.error) return res.status(built.error.status).json(built.error.payload)
		console.log('提交 Kling V3 Pro I2V 队列任务，prompt:', built.input.prompt)
		const result = await fal.queue.submit('fal-ai/kling-video/v3/pro/image-to-video', { input: built.input })
		res.status(202).json(result)
	} catch (error) {
		const status = error?.status || 500
		console.error('Kling V3 Pro I2V Submit 失败:', error?.message)
		res.status(status).json({ error: 'Kling V3 Pro 图生视频提交失败', details: error?.body?.detail || error?.body || error?.message, requestId: error?.requestId })
	}
})

// Kling V3 Pro - 图生视频（队列模式）：状态
app.get('/fal/kling-v3-pro-image-to-video/status/:requestId', apiKeyValidation, async (req, res) => {
	try {
		const status = await fal.queue.status('fal-ai/kling-video/v3/pro/image-to-video', { requestId: req.params.requestId, logs: true })
		res.json(status)
	} catch (error) {
		const statusCode = error?.status || 500
		console.error('Kling V3 Pro I2V Status 失败:', error?.message)
		res.status(statusCode).json({ error: 'Kling V3 Pro 图生视频查询状态失败', details: error?.body?.detail || error?.body || error?.message })
	}
})

// Kling V3 Pro - 图生视频（队列模式）：结果
app.get('/fal/kling-v3-pro-image-to-video/result/:requestId', apiKeyValidation, async (req, res) => {
	try {
		const result = await fal.queue.result('fal-ai/kling-video/v3/pro/image-to-video', { requestId: req.params.requestId })
		res.json(result)
	} catch (error) {
		const statusCode = error?.status || 500
		console.error('Kling V3 Pro I2V Result 失败:', error?.message)
		res.status(statusCode).json({ error: 'Kling V3 Pro 图生视频获取结果失败', details: error?.body?.detail || error?.body || error?.message })
	}
})

// Kling V3 Standard - 文生视频（同步）
app.post('/fal/kling-v3-standard', apiKeyValidation, async (req, res) => {
	try {
		const built = buildKlingV3T2VInput(req.body)
		if (built.error) return res.status(built.error.status).json(built.error.payload)
		console.log('调用 Kling V3 Standard T2V API，prompt:', built.input.prompt)
		const result = await fal.subscribe('fal-ai/kling-video/v3/standard/text-to-video', {
			input: built.input,
			logs: true,
			onQueueUpdate: (update) => { console.log('Kling V3 Standard T2V 队列状态:', update.status) },
		})
		res.json(result)
	} catch (error) {
		const status = error?.status || 500
		console.error('Kling V3 Standard T2V 调用失败:', error?.message)
		res.status(status).json({ error: 'Kling V3 Standard 文生视频失败', details: error?.body?.detail || error?.body || error?.message, requestId: error?.requestId })
	}
})

// Kling V3 Standard - 图生视频（同步）
app.post('/fal/kling-v3-standard-image-to-video', apiKeyValidation, async (req, res) => {
	try {
		const built = await buildKlingV3I2VInput(req.body)
		if (built.error) return res.status(built.error.status).json(built.error.payload)
		console.log('调用 Kling V3 Standard I2V API，prompt:', built.input.prompt)
		const result = await fal.subscribe('fal-ai/kling-video/v3/standard/image-to-video', {
			input: built.input,
			logs: true,
			onQueueUpdate: (update) => { console.log('Kling V3 Standard I2V 队列状态:', update.status) },
		})
		res.json(result)
	} catch (error) {
		const status = error?.status || 500
		console.error('Kling V3 Standard I2V 调用失败:', error?.message)
		res.status(status).json({ error: 'Kling V3 Standard 图生视频失败', details: error?.body?.detail || error?.body || error?.message, requestId: error?.requestId })
	}
})

// Flux Pro Kontext - 高质量图像生成
app.post('/fal/flux-pro-kontext', apiKeyValidation, async (req, res) => {
	try {
		const { prompt, image_size = 'landscape_4_3', num_inference_steps = 28, guidance_scale = 3.5, ...otherParams } = req.body
		
		if (!prompt) {
			return res.status(400).json({ error: 'prompt 参数是必需的' })
		}
		
		console.log('调用 Flux Pro Kontext API，prompt:', prompt)
		
		const result = await fal.subscribe('fal-ai/flux-pro/v1.1', {
			input: {
				prompt,
				image_size,
				num_inference_steps,
				guidance_scale,
				safety_tolerance: '2',
				...otherParams
			},
			logs: true,
			onQueueUpdate: (update) => {
				console.log('Flux Pro Kontext 队列状态:', update.status)
			}
		})
		
		console.log('Flux Pro Kontext 生成完成')
		res.json(result)
	} catch (error) {
		console.error('Flux Pro Kontext API 调用失败:', error)
		res.status(500).json({ 
			error: 'Flux Pro Kontext 图像生成失败', 
			details: error.message 
		})
	}
})

// Flux Pro Kontext + 自动转存到 API 服务端
// 生成图片后下载并上传到 API 服务端的 /files/upload，返回 API 服务端的 URL
app.post('/fal/flux-pro-kontext-rehost', apiKeyValidation, async (req, res) => {
	try {
		const { prompt, image_size = 'landscape_4_3', num_inference_steps = 28, guidance_scale = 3.5, api_base_url, auth_token, ...otherParams } = req.body
		
		if (!prompt) {
			return res.status(400).json({ error: 'prompt 参数是必需的' })
		}
		if (!api_base_url) {
			return res.status(400).json({ error: 'api_base_url 参数是必需的' })
		}
		if (!auth_token) {
			return res.status(400).json({ error: 'auth_token 参数是必需的' })
		}
		
		console.log('调用 Flux Pro Kontext + Rehost API，prompt:', prompt)
		
		// 1. 生成图片
		const result = await fal.subscribe('fal-ai/flux-pro/v1.1', {
			input: {
				prompt,
				image_size,
				num_inference_steps,
				guidance_scale,
				safety_tolerance: '2',
				...otherParams
			},
			logs: true,
			onQueueUpdate: (update) => {
				console.log('Flux Pro Kontext Rehost 队列状态:', update.status)
			}
		})
		
		console.log('Flux Pro Kontext 生成完成，开始转存到 API 服务端')
		console.log('Flux Pro Kontext Rehost: fal.ai 响应:', JSON.stringify(result, null, 2))
		
		// 2. 提取生成的图片 URL（支持多种响应格式）
		let imageUrl = null
		// 格式1: result.images[0].url 或 result.images[0]
		if (result?.images && Array.isArray(result.images) && result.images.length > 0) {
			const first = result.images[0]
			imageUrl = typeof first === 'string' ? first : first?.url
		}
		// 格式2: result.data.images[0].url
		if (!imageUrl && result?.data?.images && Array.isArray(result.data.images) && result.data.images.length > 0) {
			const first = result.data.images[0]
			imageUrl = typeof first === 'string' ? first : first?.url
		}
		// 格式3: result.image.url 或 result.image
		if (!imageUrl && result?.image) {
			imageUrl = typeof result.image === 'string' ? result.image : result.image?.url
		}
		// 格式4: result.output.url 或 result.output
		if (!imageUrl && result?.output) {
			imageUrl = typeof result.output === 'string' ? result.output : result.output?.url
		}
		
		if (!imageUrl) {
			console.log('Flux Pro Kontext Rehost: 未找到生成的图片 URL，响应结构:', Object.keys(result || {}))
			return res.json({ ...result, rehosted_url: null })
		}
		
		console.log('Flux Pro Kontext Rehost: 下载图片', imageUrl)
		
		// 3. 下载图片
		const imgResp = await axios.get(imageUrl, {
			responseType: 'arraybuffer',
			timeout: 30000,
		})
		const imgBuffer = Buffer.from(imgResp.data)
		const contentType = imgResp.headers['content-type'] || 'image/jpeg'
		let ext = '.jpg'
		if (contentType.includes('png')) ext = '.png'
		else if (contentType.includes('webp')) ext = '.webp'
		
		console.log('Flux Pro Kontext Rehost: 图片下载完成，大小', imgBuffer.length, '字节')
		
		// 4. 上传到 API 服务端
		const FormData = (await import('form-data')).default
		const formData = new FormData()
		formData.append('file', imgBuffer, {
			filename: `generated${ext}`,
			contentType: contentType,
		})
		
		const uploadUrl = `${api_base_url}/files/upload`
		console.log('Flux Pro Kontext Rehost: 上传到', uploadUrl)
		
		const uploadResp = await axios.post(uploadUrl, formData, {
			headers: {
				...formData.getHeaders(),
				'Authorization': `Bearer ${auth_token}`,
			},
			timeout: 30000,
		})
		
		console.log('Flux Pro Kontext Rehost: 上传响应', uploadResp.data)
		
		// 5. 构造 API 服务端的 URL
		let rehostedUrl = null
		const uploadData = uploadResp.data
		if (uploadData) {
			const fileId = uploadData.ID || uploadData.id || uploadData.file_id
			if (fileId) {
				rehostedUrl = `${api_base_url}/file/${fileId}`
			} else if (uploadData.url) {
				rehostedUrl = uploadData.url
			}
		}
		
		console.log('Flux Pro Kontext Rehost: 转存完成，URL:', rehostedUrl)
		
		res.json({
			...result,
			rehosted_url: rehostedUrl,
		})
	} catch (error) {
		console.error('Flux Pro Kontext Rehost API 调用失败:', error)
		res.status(500).json({ 
			error: 'Flux Pro Kontext Rehost 图像生成失败', 
			details: error.message 
		})
	}
})

// Flux Krea - 创意图像生成
app.post('/fal/flux-krea', apiKeyValidation, async (req, res) => {
	try {
		const { 
			prompt, 
			image_size = 'landscape_4_3',
			num_inference_steps = 28,
			guidance_scale = 3.5,
			num_images = 1,
			enable_safety_checker = true,
			seed,
			...otherParams 
		} = req.body
		
		if (!prompt) {
			return res.status(400).json({ error: 'prompt 参数是必需的' })
		}
		
		console.log('调用 Flux Krea API，prompt:', prompt)
		
		const result = await fal.subscribe('fal-ai/flux/krea', {
			input: {
				prompt,
				image_size,
				num_inference_steps,
				guidance_scale,
				num_images,
				enable_safety_checker,
				...(seed && { seed }),
				...otherParams
			},
			logs: true,
			onQueueUpdate: (update) => {
				console.log('Flux Krea 队列状态:', update.status)
			}
		})
		
		console.log('Flux Krea 生成完成')
		res.json(result)
	} catch (error) {
		console.error('Flux Krea API 调用失败:', error)
		res.status(500).json({ 
			error: 'Flux Krea 图像生成失败', 
			details: error.message 
		})
	}
})

// === 音乐生成接口 ===

// Sonauto v2 - Text to Music
app.post('/fal/sonauto-v2-text-to-music', apiKeyValidation, async (req, res) => {
	try {
		const {
			prompt,
			duration = 60,
			lyrics_prompt,
			lyrics,
			tags,
			seed,
			...otherParams
		} = req.body
		const promptText = (prompt || '').toString().trim()
		const lyricsText = (lyrics_prompt ?? lyrics ?? '').toString().trim()
		const tagList = Array.isArray(tags)
			? tags.map((t) => (t || '').toString().trim()).filter(Boolean)
			: (tags || '')
				.toString()
				.split(',')
				.map((t) => t.trim())
				.filter(Boolean)

		const hasPrompt = !!promptText
		const hasTags = tagList.length > 0
		const hasLyrics = !!lyricsText
		if (!hasPrompt && !hasTags && !hasLyrics) {
			return res.status(400).json({ error: '至少提供 prompt/tags/lyrics_prompt 之一' })
		}
		if (hasLyrics && !hasPrompt && !hasTags) {
			return res.status(400).json({ error: '仅提供 lyrics_prompt 不足，请至少再提供 prompt 或 tags' })
		}
		if (hasPrompt && hasTags && hasLyrics) {
			return res.status(400).json({ error: '请不要同时提供 prompt、tags、lyrics_prompt 三个字段' })
		}

		console.log('调用 Sonauto v2 API，prompt:', promptText || '(empty)')

		const result = await fal.subscribe('sonauto/v2/text-to-music', {
			input: {
				...(hasPrompt ? { prompt: promptText } : {}),
				...(hasTags ? { tags: tagList } : {}),
				...(hasLyrics ? { lyrics_prompt: lyricsText } : {}),
				duration,
				...(seed !== undefined && seed !== null && seed !== '' ? { seed } : {}),
				...otherParams,
			},
			logs: true,
			onQueueUpdate: (update) => {
				console.log('Sonauto v2 队列状态:', update.status)
			},
		})

		console.log('Sonauto v2 生成完成')
		res.json(result)
	} catch (error) {
		console.error('Sonauto v2 API 调用失败:', error)
		const status = error?.status || 500
		const requestId = error?.requestId
		const body = error?.body
		const detail = body?.detail
		res.status(status).json({
			error: 'Sonauto v2 歌曲生成失败',
			details: detail || body || error?.message,
			status,
			requestId,
		})
	}
})

// MiniMax Music v2
app.post('/fal/minimax-music-v2', apiKeyValidation, async (req, res) => {
	try {
		const {
			prompt,
			duration = 60,
			lyrics_prompt,
			lyrics,
			audio_setting,
			tags,
			seed,
			...otherParams
		} = req.body
		const promptText = (prompt || '').toString().trim()
		const lyricsText = (lyrics_prompt ?? lyrics ?? '').toString().trim()

		if (!promptText) {
			return res.status(400).json({ error: 'prompt 参数是必需的' })
		}
		if (!lyricsText) {
			return res.status(400).json({ error: 'lyrics_prompt 参数是必需的' })
		}

		console.log('调用 MiniMax Music v2 API，prompt:', promptText)

		const result = await fal.subscribe('fal-ai/minimax-music/v2', {
			input: {
				prompt: promptText,
				lyrics_prompt: lyricsText,
				...(audio_setting ? { audio_setting } : {}),
				duration,
				...(tags ? { tags } : {}),
				...(seed !== undefined && seed !== null && seed !== '' ? { seed } : {}),
				...otherParams,
			},
			logs: true,
			onQueueUpdate: (update) => {
				console.log('MiniMax Music v2 队列状态:', update.status)
			},
		})

		console.log('MiniMax Music v2 生成完成')
		res.json(result)
	} catch (error) {
		console.error('MiniMax Music v2 API 调用失败:', error)
		const status = error?.status || 500
		const requestId = error?.requestId
		const body = error?.body
		const detail = body?.detail
		res.status(status).json({
			error: 'MiniMax Music v2 歌曲生成失败',
			details: detail || body || error?.message,
			status,
			requestId,
		})
	}
})

// MusicGen Stereo Large - 立体声音乐生成
app.post('/fal/musicgen-stereo', apiKeyValidation, async (req, res) => {
	try {
		const { 
			prompt, 
			duration = 10,
			temperature = 1.0,
			top_k = 250,
			top_p = 0.0,
			classifier_free_guidance = 3.0,
			...otherParams 
		} = req.body
		
		if (!prompt) {
			return res.status(400).json({ error: 'prompt 参数是必需的' })
		}
		
		console.log('调用 MusicGen Stereo API，prompt:', prompt)
		
		const result = await fal.subscribe('fal-ai/musicgen/stereo-large', {
			input: {
				prompt,
				duration,
				temperature,
				top_k,
				top_p,
				classifier_free_guidance,
				...otherParams
			},
			logs: true,
			onQueueUpdate: (update) => {
				console.log('MusicGen Stereo 队列状态:', update.status)
			}
		})
		
		console.log('MusicGen Stereo 生成完成')
		res.json(result)
	} catch (error) {
		console.error('MusicGen Stereo API 调用失败:', error)
		res.status(500).json({ 
			error: 'MusicGen Stereo 音乐生成失败', 
			details: error.message 
		})
	}
})

// ACE-Step - 歌词转歌曲生成（支持人声）
app.post('/fal/ace-step', apiKeyValidation, async (req, res) => {
	try {
		const {
			lyrics = '',
			tags = 'pop, vocal, emotional',
			duration = 60,
			instrumental = false,
			seed,
			...otherParams
		} = req.body

		if (!lyrics && !instrumental) {
			return res.status(400).json({ error: 'lyrics 参数是必需的（除非生成纯器乐）' })
		}

		console.log('调用 ACE-Step API，tags:', tags, 'duration:', duration)

		const result = await fal.subscribe('fal-ai/ace-step', {
			input: {
				lyrics,
				tags,
				duration,
				instrumental,
				...(seed !== undefined && seed !== null && seed !== '' ? { seed } : {}),
				...otherParams,
			},
			logs: true,
			onQueueUpdate: (update) => {
				console.log('ACE-Step 队列状态:', update.status)
			},
		})

		console.log('ACE-Step 生成完成')
		res.json(result)
	} catch (error) {
		console.error('ACE-Step API 调用失败:', error)
		const status = error?.status || 500
		const requestId = error?.requestId
		const body = error?.body
		const detail = body?.detail
		res.status(status).json({
			error: 'ACE-Step 歌曲生成失败',
			details: detail || body || error?.message,
			status,
			requestId,
		})
	}
})

// Stable Audio - 高质量音频生成
app.post('/fal/stable-audio', apiKeyValidation, async (req, res) => {
	try {
		const { 
			prompt, 
			duration = 30,
			negative_prompt = '',
			seed,
			...otherParams 
		} = req.body
		
		if (!prompt) {
			return res.status(400).json({ error: 'prompt 参数是必需的' })
		}
		
		console.log('调用 Stable Audio API，prompt:', prompt)
		
		const result = await fal.subscribe('fal-ai/stable-audio', {
			input: {
				prompt,
				duration,
				negative_prompt,
				...(seed && { seed }),
				...otherParams
			},
			logs: true,
			onQueueUpdate: (update) => {
				console.log('Stable Audio 队列状态:', update.status)
			}
		})
		
		console.log('Stable Audio 生成完成')
		res.json(result)
	} catch (error) {
		console.error('Stable Audio API 调用失败:', error)
		res.status(500).json({ 
			error: 'Stable Audio 音频生成失败', 
			details: error.message 
		})
	}
})

app.post('/fal/beatoven-sound-effect', apiKeyValidation, async (req, res) => {
	try {
		const {
			prompt,
			negative_prompt = '',
			duration = 5,
			refinement = 40,
			creativity = 16,
			seed,
			...otherParams
		} = req.body

		if (!prompt) {
			return res.status(400).json({ error: 'prompt 参数是必需的' })
		}

		console.log('调用 Beatoven Sound Effect API，prompt:', prompt)

		const result = await fal.subscribe('beatoven/sound-effect-generation', {
			input: {
				prompt,
				negative_prompt,
				duration,
				refinement,
				creativity,
				...(seed !== undefined && seed !== null && seed !== '' ? { seed } : {}),
				...otherParams,
			},
			logs: true,
			onQueueUpdate: (update) => {
				console.log('Beatoven Sound Effect 队列状态:', update.status)
			},
		})

		console.log('Beatoven Sound Effect 生成完成')
		res.json(result)
	} catch (error) {
		console.error('Beatoven Sound Effect API 调用失败:', error)
		const status = error?.status || 500
		const requestId = error?.requestId
		const body = error?.body
		const detail = body?.detail
		res.status(status).json({
			error: 'Beatoven 音效生成失败',
			details: detail || body || error?.message,
			status,
			requestId,
		})
	}
})

app.post('/fal/eleven-v3-tts', apiKeyValidation, async (req, res) => {
	try {
		const {
			text,
			voice = 'Rachel',
			stability = 0.5,
			similarity_boost = 0.75,
			style,
			speed = 1,
			timestamps,
			language_code,
			apply_text_normalization = 'auto',
			...otherParams
		} = req.body

		if (!text) {
			return res.status(400).json({ error: 'text 参数是必需的' })
		}

		console.log('调用 Eleven V3 TTS API，text:', text)

		const result = await fal.subscribe('fal-ai/elevenlabs/tts/eleven-v3', {
			input: {
				text,
				voice,
				stability,
				similarity_boost,
				...(style !== undefined && style !== null ? { style } : {}),
				speed,
				...(timestamps !== undefined && timestamps !== null
					? { timestamps }
					: {}),
				...(language_code ? { language_code } : {}),
				apply_text_normalization,
				...otherParams,
			},
			logs: true,
			onQueueUpdate: (update) => {
				console.log('Eleven V3 TTS 队列状态:', update.status)
			},
		})

		console.log('Eleven V3 TTS 生成完成')
		res.json(result)
	} catch (error) {
		console.error('Eleven V3 TTS API 调用失败:', error)
		const status = error?.status || 500
		const requestId = error?.requestId
		const body = error?.body
		const detail = body?.detail
		res.status(status).json({
			error: 'Eleven V3 TTS 生成失败',
			details: detail || body || error?.message,
			status,
			requestId,
		})
	}
})

app.post('/fal/minimax-speech-02-hd', apiKeyValidation, async (req, res) => {
	try {
		const {
			text,
			voice_setting,
			audio_setting,
			language_boost,
			output_format = 'url',
			pronunciation_dict,
			...otherParams
		} = req.body

		if (!text || !text.toString().trim()) {
			return res.status(400).json({ error: 'text 参数是必需的' })
		}

		console.log('调用 MiniMax Speech-02-HD API，text:', text)

		const result = await fal.subscribe('fal-ai/minimax/speech-02-hd', {
			input: {
				text,
				...(voice_setting ? { voice_setting } : {}),
				...(audio_setting ? { audio_setting } : {}),
				...(language_boost ? { language_boost } : {}),
				...(output_format ? { output_format } : {}),
				...(pronunciation_dict ? { pronunciation_dict } : {}),
				...otherParams,
			},
			logs: true,
			onQueueUpdate: (update) => {
				console.log('MiniMax Speech-02-HD 队列状态:', update.status)
			},
		})

		console.log('MiniMax Speech-02-HD 生成完成')
		res.json(result)
	} catch (error) {
		console.error('MiniMax Speech-02-HD API 调用失败:', error)
		const status = error?.status || 500
		const requestId = error?.requestId
		const body = error?.body
		const detail = body?.detail
		res.status(status).json({
			error: 'MiniMax Speech-02-HD 生成失败',
			details: detail || body || error?.message,
			status,
			requestId,
		})
	}
})

// Talking-head / avatar video (generic) - image + audio -> video
app.post('/fal/talking-head', apiKeyValidation, async (req, res) => {
	try {
		const model = (req.body?.model || '').toString().trim()
		const inputRaw = req.body?.input || {}
		if (!model) {
			return res.status(400).json({ error: 'model 参数是必需的' })
		}
		if (!inputRaw || typeof inputRaw !== 'object') {
			return res.status(400).json({ error: 'input 参数是必需的' })
		}

		const input = { ...inputRaw }
		if (input.image_url) {
			input.image_url = await ensureFalAccessibleFileUrl(input.image_url)
		}
		if (input.audio_url) {
			input.audio_url = await ensureFalAccessibleFileUrl(input.audio_url)
		}
		if (input.audioUrl && !input.audio_url) {
			input.audioUrl = await ensureFalAccessibleFileUrl(input.audioUrl)
		}

		console.log('调用 Fal talking-head, model:', model)
		const result = await fal.subscribe(model, {
			input,
			logs: true,
			onQueueUpdate: (update) => {
				console.log('talking-head 队列状态:', update.status)
			},
		})
		return res.status(200).json(result)
	} catch (error) {
		console.error('Fal talking-head 调用失败:', error)
		const status = error?.status || 500
		const requestId = error?.requestId
		const body = error?.body
		const detail = body?.detail
		return res.status(status).json({
			error: 'talking-head 生成失败',
			details: detail || body || error?.message,
			status,
			requestId,
		})
	}
})

// 获取所有 fal.ai 接口信息
app.get('/fal/models', apiKeyValidation, (req, res) => {
	try {
		const models = {
			video_generation: {
				'veo3-fast': {
					name: 'Veo 3 Fast',
					description: '快速文本生成视频，成本更低',
					endpoint: '/fal/veo3-fast',
					type: 'text-to-video',
					max_duration: 8,
					supported_ratios: ['16:9', '9:16', '1:1']
				},
				'veo3': {
					name: 'Veo 3',
					description: '高质量文本生成视频',
					endpoint: '/fal/veo3',
					type: 'text-to-video',
					max_duration: 8,
					supported_ratios: ['16:9', '9:16', '1:1']
				},
				'veo2-image-to-video': {
					name: 'Veo 2 Image-to-Video',
					description: '图片生成视频',
					endpoint: '/fal/veo2-image-to-video',
					type: 'image-to-video',
					max_duration: 8,
					supported_ratios: ['16:9', '9:16', '1:1']
				},
				'veo3-image-to-video': {
					name: 'Veo 3 Image-to-Video',
					description: 'Veo 3图像生成视频，高质量效果',
					endpoint: '/fal/veo3-image-to-video',
					type: 'image-to-video',
					max_duration: 8,
					supported_ratios: ['16:9', '9:16', '1:1'],
					features: ['motion_control', 'high_quality']
				},
				'veo3-fast-image-to-video': {
					name: 'Veo 3 Fast Image-to-Video',
					description: 'Veo 3快速图像生成视频，成本更低',
					endpoint: '/fal/veo3-fast-image-to-video',
					type: 'image-to-video',
					max_duration: 8,
					supported_ratios: ['16:9', '9:16', '1:1'],
					features: ['motion_control', 'fast_generation']
				},
				'kling-v3-pro': {
					name: 'Kling V3 Pro',
					description: '可灵 V3 Pro 文生视频，电影级画质，支持原生音频生成，3-15秒',
					endpoint: '/fal/kling-v3-pro',
					type: 'text-to-video',
					max_duration: 15,
					supported_ratios: ['16:9', '9:16', '1:1'],
					features: ['native_audio', 'cfg_scale', 'negative_prompt', 'multi_prompt']
				},
				'kling-v3-pro-image-to-video': {
					name: 'Kling V3 Pro Image-to-Video',
					description: '可灵 V3 Pro 图生视频，支持首尾帧、原生音频，3-15秒',
					endpoint: '/fal/kling-v3-pro-image-to-video',
					type: 'image-to-video',
					max_duration: 15,
					supported_ratios: ['16:9', '9:16', '1:1'],
					features: ['native_audio', 'start_end_image', 'elements', 'cfg_scale']
				},
				'kling-v3-standard': {
					name: 'Kling V3 Standard',
					description: '可灵 V3 Standard 文生视频，性价比高，3-15秒',
					endpoint: '/fal/kling-v3-standard',
					type: 'text-to-video',
					max_duration: 15,
					supported_ratios: ['16:9', '9:16', '1:1'],
					features: ['native_audio', 'cfg_scale', 'cost_effective']
				},
				'kling-v3-standard-image-to-video': {
					name: 'Kling V3 Standard Image-to-Video',
					description: '可灵 V3 Standard 图生视频，性价比高，3-15秒',
					endpoint: '/fal/kling-v3-standard-image-to-video',
					type: 'image-to-video',
					max_duration: 15,
					supported_ratios: ['16:9', '9:16', '1:1'],
					features: ['native_audio', 'start_end_image', 'cost_effective']
				}
			},
			image_generation: {
				'flux-pro-kontext': {
					name: 'Flux Pro Kontext',
					description: '高质量图像生成',
					endpoint: '/fal/flux-pro-kontext',
					type: 'text-to-image',
					supported_sizes: ['square_hd', 'square', 'portrait_4_3', 'portrait_16_9', 'landscape_4_3', 'landscape_16_9']
				},
				'flux-krea': {
					name: 'Flux Krea',
					description: '创意图像生成，支持多种艺术风格',
					endpoint: '/fal/flux-krea',
					type: 'text-to-image',
					supported_sizes: ['square_hd', 'square', 'portrait_4_3', 'portrait_16_9', 'landscape_4_3', 'landscape_16_9'],
					features: ['creative_styles', 'artistic_generation', 'style_transfer']
				}
			},
			audio_generation: {
				'ace-step': {
					name: 'ACE-Step',
					description: '歌词转歌曲生成，支持人声和器乐',
					endpoint: '/fal/ace-step',
					type: 'lyrics-to-song',
					max_duration: 300,
					features: ['lyrics_input', 'vocal_generation', 'instrumental_mode', 'genre_tags', 'seed']
				},
				'musicgen-stereo': {
					name: 'MusicGen Stereo Large',
					description: '立体声音乐生成，支持多种音乐风格',
					endpoint: '/fal/musicgen-stereo',
					type: 'text-to-music',
					max_duration: 30,
					features: ['stereo_output', 'music_generation', 'style_control']
				},
				'beatoven-sound-effect': {
					name: 'Beatoven Sound Effect',
					description: '高质量音效生成（Sound Effect Generation）',
					endpoint: '/fal/beatoven-sound-effect',
					type: 'text-to-audio',
					max_duration: 35,
					features: ['sound_effects', 'wav_output', 'seed', 'negative_prompt']
				},
				'stable-audio': {
					name: 'Stable Audio',
					description: '高质量音频和音乐生成',
					endpoint: '/fal/stable-audio',
					type: 'text-to-audio',
					max_duration: 90,
					features: ['high_quality', 'long_duration', 'negative_prompts']
				},
				'eleven-v3-tts': {
					name: 'Eleven V3 TTS',
					description: '高质量文本转语音（ElevenLabs）',
					endpoint: '/fal/eleven-v3-tts',
					type: 'text-to-speech',
					max_duration: 0,
					features: ['tts', 'voice', 'timestamps', 'language_code']
				},
				'minimax-speech-02-hd': {
					name: 'MiniMax Speech-02-HD',
					description: '高质量文本转语音（MiniMax HD）',
					endpoint: '/fal/minimax-speech-02-hd',
					type: 'text-to-speech',
					max_duration: 0,
					features: ['tts', 'voice_setting', 'audio_setting', 'language_boost']
				}
			}
		}
		
		res.json({
			models,
			total_models: Object.keys(models.video_generation).length + Object.keys(models.image_generation).length + Object.keys(models.audio_generation).length,
			categories: ['video_generation', 'image_generation', 'audio_generation']
		})
	} catch (error) {
		console.error('获取 fal.ai 模型列表失败:', error)
		res.status(500).json({ error: '获取模型列表失败' })
	}
})

app.post('/create-speech', async (req, res) => {
	try {
		const { model, voice, input } = req.body
		const response = await openai.audio.speech.create({
			model: model || 'tts-1',
			voice: voice || 'alloy',
			input: input,
		})

		// 使用当前日期和时间生成唯一的文件名
		const now = new Date()
		const timestamp = now.toISOString().replace(/:/g, '-').replace(/\..+/, '')
		const speechFileName = `speech_${timestamp}.mp3`
		const speechFilePath = path.resolve(path.join(AUDIO_DIR, speechFileName))

		if (!fs.existsSync(AUDIO_DIR)) {
			fs.mkdirSync(AUDIO_DIR, { recursive: true })
		}

		// 将响应的音频数据写入文件
		const buffer = Buffer.from(await response.arrayBuffer())
		await fs.promises.writeFile(speechFilePath, buffer)

		// 发送MP3文件路径作为响应
		res.send({ message: 'Speech created successfully.', path: speechFilePath })
	} catch (error) {
		console.error(error)
		// 发送具体的错误消息和错误堆栈（如果可用）
		res.status(500).send({
			error: 'Error creating speech.',
			reason: error.message, // 这里提供了错误的具体原因
			stack: error.stack, // 可选地，也可以提供错误堆栈以帮助调试
		})
	}
})

app.post('/post-transcription', upload.single('audio'), async (req, res) => {
	console.log('请求收到，开始处理')

	if (!req.file) {
		console.error('没有上传音频文件')
		return res.status(400).send('No audio file uploaded.')
	}

	console.log('文件上传成功:', req.file)

	// 从请求体中提取其他参数
	const { model, response_format, timestamp_granularities } = req.body
	console.log('请求参数:', { model, response_format, timestamp_granularities })

	try {
		// 获取原始文件名的扩展名
		const originalExtension = path.extname(req.file.originalname)
		const filePath = path.resolve(req.file.path)
		const newFilePath = filePath + originalExtension
		fs.renameSync(filePath, newFilePath)
		console.log('重命名后的文件路径:', newFilePath)

		// 确保文件格式正确
		const supportedFormats = [
			'flac',
			'm4a',
			'mp3',
			'mp4',
			'mpeg',
			'mpga',
			'oga',
			'ogg',
			'wav',
			'webm',
		]
		const fileExtension = originalExtension.substring(1)

		if (!supportedFormats.includes(fileExtension)) {
			console.error('不支持的文件格式:', fileExtension)
			fs.unlinkSync(newFilePath) // 删除不支持格式的文件
			return res
				.status(400)
				.send(
					`Unsupported file format. Supported formats: ${supportedFormats.join(
						', '
					)}`
				)
		}

		// 创建音频转录
		const transcriptionResponse = await openai.audio.transcriptions.create({
			file: fs.createReadStream(newFilePath),
			model: model,
			response_format: response_format,
			timestamp_granularities: timestamp_granularities
				? JSON.parse(timestamp_granularities)
				: undefined,
		})

		console.log('转录响应:', transcriptionResponse)

		// 清理上传的文件
		fs.unlinkSync(newFilePath)
		console.log('临时文件已删除')

		// 返回转录结果的TEXT
		res.json(transcriptionResponse.text)
	} catch (error) {
		console.error('转录出错:', error)
		res.status(500).send('Error creating transcription.')
	}
})

app.post('/create-transcription', upload.single('audio'), async (req, res) => {
	if (!req.file) {
		return res.status(400).send('No audio file uploaded.')
	}

	// 从请求体中提取其他参数
	const { model, response_format, timestamp_granularities } = req.body

	try {
		// 读取上传的音频文件
		const filePath = path.resolve(req.file.path)

		// 创建音频转录
		const transcriptionResponse = await openai.audio.transcriptions.create({
			file: fs.createReadStream(filePath),
			model: model,
			response_format: response_format,
			timestamp_granularities: timestamp_granularities
				? JSON.parse(timestamp_granularities)
				: undefined,
		})

		// 清理上传的文件
		fs.unlinkSync(filePath)

		// 返回转录结果
		res.json(transcriptionResponse.data)
	} catch (error) {
		console.error(error)
		res.status(500).send('Error creating transcription.')
	}
})

app.post('/create-translation', upload.single('audio'), async (req, res) => {
	if (!req.file) {
		return res.status(400).send('No audio file uploaded.')
	}

	try {
		// 请在请求中指定模型名称，例如 "whisper-1"
		const { model } = req.body

		// 读取上传的音频文件并创建翻译
		const filePath = path.resolve(req.file.path)
		const translationResponse = await openai.audio.translations.create({
			file: fs.createReadStream(filePath),
			model: model || 'whisper-1',
		})

		// 清理上传的文件
		fs.unlinkSync(filePath)

		// 返回翻译结果
		res.json({ translation: translationResponse.data })
	} catch (error) {
		console.error(error)
		res.status(500).send('Error creating translation.')
	}
})

app.post('/chat-completion', upload.single('file'), async (req, res) => {
	let isAborted = false

	// 创建 AbortController 实例
	const controller = new AbortController()
	const { signal } = controller

	// 监听请求中止事件
	req.on('aborted', () => {
		console.log('客户端已取消请求')
		isAborted = true
		controller.abort() // 中止 OpenAI 请求
	})

	try {
		const { model, messages, stream, stop } = req.body
		// 当 stream 为 true 时，将 streamType 设置为 'sse'
		const streamType = stream ? 'sse' : req.body.streamType

		// 构建请求参数，只有在 stop 提供时才包括它
		// 构建请求参数，只有在 stream 为 true 时才添加 stream_options
		const params = {
			model,
			messages,
			stream: Boolean(stream),
		}

		if (params.stream) {
			params.stream_options = {
				include_usage: true,
			}
		}

		if (stop) {
			params.stop = stop
		}

		// 如果有文件，处理文件
		let attachment = null
		if (req.file) {
			const filePath = path.join(__dirname, 'uploads', req.file.filename)
			const fileUrl = `/uploads/${req.file.filename}` // 你需要配置静态文件服务
			attachment = {
				url: fileUrl,
				filename: req.file.originalname,
				mimetype: req.file.mimetype,
			}

			// 你可以根据需求将附件信息添加到消息中
			// 例如，添加到最后一条用户消息
			if (params.messages && params.messages.length > 0) {
				params.messages[params.messages.length - 1].attachment = attachment
			}
		}

		console.log('接收到的请求参数:', params)

		// 发起 OpenAI API 请求，确保使用正确的方法名并设置 responseType 为 stream
		const openaiResponse = await openai.chat.completions.create(params, {
			signal,
			responseType: 'stream', // 确保响应类型为流
		})

		// 添加详细日志以检查 openaiResponse 的结构
		console.log('openaiResponse:', openaiResponse)
		console.log('openaiResponse 类型:', typeof openaiResponse)
		console.log(
			'openaiResponse 是否为 Async Iterator:',
			typeof openaiResponse[Symbol.asyncIterator] === 'function'
		)

		if (stream) {
			// 直接使用 openaiResponse 作为异步迭代器
			const streamResponse = openaiResponse

			if (!streamResponse) {
				console.error('OpenAI 流式响应 undefined')
				return res.status(500).send('OpenAI 流式响应 undefined')
			}

			// 检查是否为异步迭代器
			if (typeof streamResponse[Symbol.asyncIterator] !== 'function') {
				console.error('streamResponse 不是一个异步迭代器')
				console.error('streamResponse:', streamResponse)
				return res.status(500).send('streamResponse 不是一个异步迭代器')
			}

			if (streamType === 'sse') {
				// 设置响应头为SSE
				res.setHeader('Content-Type', 'text/event-stream')
				res.setHeader('Cache-Control', 'no-cache')
				res.setHeader('Connection', 'keep-alive')
				res.flushHeaders() // 立即发送响应头

				console.log('使用SSE进行流式响应')

				// 使用异步迭代器处理流式响应
				for await (const chunk of streamResponse) {
					if (isAborted) {
						console.log('检测到请求取消，停止发送数据')
						break
					}

					console.log('接收到的数据块:', chunk)
					// 将数据块序列化为 JSON 字符串后发送
					// res.write(`data: ${JSON.stringify(chunk)}\n\n`)
					if (chunk.usage && chunk.usage.total_tokens) {
						console.log('接收到的 usage 数据:', chunk.usage)
						// 将 usage 数据发送给客户端
						res.write(`data: ${JSON.stringify({ usage: chunk.usage })}\n\n`)
					} else if (chunk.choices && chunk.choices.length > 0) {
						// 发送生成的内容
						res.write(`data: ${JSON.stringify(chunk)}\n\n`)
					}

					// 检查是否达到停止条件
					// try {
					// 	if (chunk.choices && chunk.choices[0].finish_reason === 'stop') {
					// 		console.log('生成内容已达停止条件')
					// 		break
					// 	}
					// } catch (e) {
					// 	console.error('解析数据时出错:', e)
					// }
				}

				if (!isAborted) {
					console.log('数据流结束')
					// 发送结束标志
					res.write('data: [DONE]\n\n')
					res.end()
				} else {
					console.log('由于请求取消，提前结束响应')
					res.end()
				}
			} else {
				// 默认使用普通JSON流式响应
				res.setHeader('Content-Type', 'application/json')
				res.setHeader('Cache-Control', 'no-cache')
				res.setHeader('Connection', 'keep-alive')
				res.flushHeaders() // 立即发送响应头

				console.log('使用普通JSON流式响应')

				// 使用异步迭代器处理流式响应
				for await (const chunk of streamResponse) {
					if (isAborted) {
						console.log('检测到请求取消，停止发送数据')
						break
					}

					console.log('接收到的数据块:', chunk)
					// 将数据块序列化为 JSON 字符串后发送
					res.write(`${JSON.stringify(chunk)}\n`) // 使用单个换行符分隔
				}

				if (!isAborted) {
					console.log('数据流结束')
					res.end()
				} else {
					console.log('由于请求取消，提前结束响应')
					res.end()
				}
			}
		} else {
			// 非流式响应
			if (!openaiResponse) {
				// 修改此处，去除 .data
				console.error('OpenAI 非流式响应 undefined')
				return res.status(500).send('OpenAI 非流式响应 undefined')
			}

			console.log('OpenAI 非流式响应:', openaiResponse)
			res.json(openaiResponse) // 修改此处，去除 .data
		}
	} catch (error) {
		if (isAborted) {
			console.log('请求已被中止，无需返回错误响应')
		} else {
			console.error('服务器端错误:', error)
			if (!res.headersSent) {
				res.status(500).send('创建聊天完成时出错。')
			}
		}
	}
})

app.post('/chat', apiKeyValidation, async (req, res) => {
	let isAborted = false
	const controller = new AbortController()
	const { signal } = controller

	req.on('aborted', () => {
		isAborted = true
		controller.abort()
	})

	try {
		const {
			model = 'gpt-4o-mini',
			messages,
			stream = false,
			temperature,
			max_tokens,
			stop,
		} = req.body || {}

		if (!Array.isArray(messages) || messages.length === 0) {
			return res.status(400).json({ error: 'messages is required' })
		}

		const params = {
			model,
			messages,
			stream: Boolean(stream),
			...(temperature !== undefined ? { temperature } : {}),
			...(max_tokens !== undefined ? { max_tokens } : {}),
			...(stop ? { stop } : {}),
		}

		const openaiResponse = await openai.chat.completions.create(params, {
			signal,
			responseType: params.stream ? 'stream' : undefined,
		})

		if (params.stream) {
			if (!openaiResponse || typeof openaiResponse[Symbol.asyncIterator] !== 'function') {
				return res.status(500).json({ error: 'OpenAI stream response invalid' })
			}

			res.setHeader('Content-Type', 'text/event-stream')
			res.setHeader('Cache-Control', 'no-cache')
			res.setHeader('Connection', 'keep-alive')
			res.setHeader('X-Accel-Buffering', 'no')
			res.flushHeaders()

			for await (const chunk of openaiResponse) {
				if (isAborted) break
				res.write(`data: ${JSON.stringify(chunk)}\n\n`)
			}

			if (!isAborted) {
				res.write('data: [DONE]\n\n')
			}
			return res.end()
		}

		return res.json(openaiResponse)
	} catch (error) {
		if (error?.name === 'AbortError') {
			return res.status(499).json({ error: '请求被中止' })
		}
		return res.status(500).json({ error: 'OpenAI Chat 失败', details: error?.message })
	}
})

app.post('/realtime/client-secrets', apiKeyValidation, async (req, res) => {
	try {
		const raw = req.body || {}
		const sessionRaw = raw.session && typeof raw.session === 'object' ? raw.session : raw
		if (!sessionRaw || typeof sessionRaw !== 'object') {
			return res.status(400).json({ error: 'session config is required' })
		}

		const session = {
			...sessionRaw,
			type: (sessionRaw.type || 'realtime').toString(),
			model: (sessionRaw.model || 'gpt-realtime').toString(),
		}
		if (session.type !== 'realtime' && session.type !== 'transcription') {
			return res.status(400).json({ error: 'invalid session.type' })
		}
		if (session.type === 'realtime') {
			session.audio = session.audio && typeof session.audio === 'object' ? session.audio : {}
			session.audio.output =
				session.audio.output && typeof session.audio.output === 'object'
					? session.audio.output
					: {}
			if (!session.audio.output.voice) {
				session.audio.output.voice = 'marin'
			}
		}

		const upstream = await axios.post(
			'https://api.openai.com/v1/realtime/client_secrets',
			{ session },
			{
				headers: {
					Authorization: `Bearer ${API_KEY}`,
					'Content-Type': 'application/json',
				},
				timeout: 30 * 1000,
			}
		)

		return res.json(upstream.data)
	} catch (error) {
		const status = error?.response?.status || 500
		const details =
			error?.response?.data ||
			error?.message ||
			'Realtime client_secrets failed'
		return res.status(status).json({ error: 'Realtime client_secrets 失败', details })
	}
})

app.post('/create-embedding', async (req, res) => {
	const { model, input, encoding_format } = req.body

	try {
		// 确保请求中包含了必要的参数
		if (!model || !input) {
			return res.status(400).send('Model and input are required.')
		}

		const embeddingResponse = await openai.embeddings.create({
			model,
			input,
			encoding_format: encoding_format || 'float', // 默认使用'float'作为编码格式
		})

		// 将嵌入结果发送回客户端
		res.json(embeddingResponse.data)
	} catch (error) {
		console.error(error)
		res.status(500).send({ error: 'Error creating embedding.' })
	}
})

app.post('/create-fine-tune', async (req, res) => {
	const { training_file, model, hyperparameters, validation_file } = req.body

	try {
		// 检查必要的参数
		if (!training_file) {
			return res.status(400).send('Training file is required.')
		}

		const fineTuneResponse = await openai.fineTuning.jobs.create({
			training_file,
			...(model && { model }), // 如果提供了model参数，则添加到请求中
			...(hyperparameters && { hyperparameters }), // 如果提供了hyperparameters，则添加到请求中
			...(validation_file && { validation_file }), // 如果提供了validation_file，则添加到请求中
		})

		// 将微调作业的创建结果发送回客户端
		res.json(fineTuneResponse.data)
	} catch (error) {
		console.error(error)
		res.status(500).send({ error: 'Error creating fine-tune job.' })
	}
})

app.get('/list-fine-tune-jobs', async (req, res) => {
	try {
		const response = await openai.fineTuning.jobs.list()

		// 初始化一个数组来收集所有微调作业
		const fineTunes = []

		// 如果返回的数据是异步迭代器，则遍历它以收集作业
		if (Symbol.asyncIterator in response) {
			for await (const fineTune of response) {
				fineTunes.push(fineTune)
			}
		} else {
			// 如果数据直接以列表形式返回，则直接使用
			fineTunes.push(...response.data)
		}

		// 将微调作业列表发送回客户端
		res.json(fineTunes)
	} catch (error) {
		console.error(error)
		res.status(500).send({ error: 'Error listing fine-tune jobs.' })
	}
})

// 新增路由以列出特定微调作业的事件
app.get('/list-fine-tune-events', async (req, res) => {
	// 从查询参数中获取微调作业的ID和限制参数
	const { id, limit } = req.query

	// 检查是否提供了微调作业的ID
	if (!id) {
		return res.status(400).send('Fine-tune job ID is required.')
	}

	try {
		// 调用SDK以获取特定微调作业的事件
		const list = await openai.fineTuning.list_events({
			id: id,
			...(limit && { limit: parseInt(limit, 10) }), // 如果提供了限制参数，则添加到请求中
		})

		const events = []

		// 遍历异步迭代器以收集事件
		if (Symbol.asyncIterator in list) {
			for await (const event of list) {
				events.push(event)
			}
		} else {
			// 如果数据直接以列表形式返回，则直接使用
			events.push(...list.data)
		}

		// 将微调作业事件列表发送回客户端
		res.json(events)
	} catch (error) {
		console.error(error)
		res.status(500).send({ error: 'Error listing fine-tune events.' })
	}
})

// 新增路由以检索特定微调作业的详情
app.get('/retrieve-fine-tune-job/:jobId', async (req, res) => {
	// 从路径参数中获取微调作业的ID
	const jobId = req.params.jobId

	try {
		// 调用SDK以获取特定微调作业的详情
		const fineTune = await openai.fineTuning.jobs.retrieve(jobId)

		// 将微调作业的详情发送回客户端
		res.json(fineTune.data)
	} catch (error) {
		console.error(error)
		res.status(500).send({ error: 'Error retrieving fine-tune job.' })
	}
})

// 新增路由以取消特定微调作业
app.post('/cancel-fine-tune-job/:jobId', async (req, res) => {
	// 从路径参数中获取微调作业的ID
	const jobId = req.params.jobId

	try {
		// 调用SDK以取消特定微调作业
		const fineTune = await openai.fineTuning.jobs.cancel(jobId)

		// 将取消操作的结果发送回客户端
		res.json(fineTune.data)
	} catch (error) {
		console.error(error)
		res.status(500).send({ error: 'Error canceling fine-tune job.' })
	}
})

app.post('/upload-file', upload.single('file'), async (req, res) => {
	if (!req.file) {
		return res.status(400).send('未上传文件。')
	}

	// 获取客户端提供的 purpose 参数
	const { purpose } = req.body

	if (!purpose) {
		return res.status(400).send('必须提供用途参数。')
	}

	// 获取上传文件的扩展名
	const fileExtension = path.extname(req.file.originalname).toLowerCase()

	// 根据不同的用途检查文件类型和大小
	if (purpose === 'fine-tune' || purpose === 'batch') {
		if (fileExtension !== '.jsonl') {
			fs.unlinkSync(req.file.path) // 删除不符合要求的文件
			return res
				.status(400)
				.send('用于 fine-tune 或 batch 的文件必须是 .jsonl 格式。')
		}
		if (purpose === 'batch') {
			const fileSize = fs.statSync(req.file.path).size
			if (fileSize > 100 * 1024 * 1024) {
				fs.unlinkSync(req.file.path) // 删除不符合要求的文件
				return res.status(400).send('Batch API 的文件大小不能超过 100 MB。')
			}
		}
	}

	if (purpose === 'assistants') {
		const supportedAssistantsFormats = [
			'.doc',
			'.docx',
			'.pdf',
			'.jpg',
			'.jpeg',
			'.png',
		]
		if (!supportedAssistantsFormats.includes(fileExtension)) {
			fs.unlinkSync(req.file.path) // 删除不符合要求的文件
			return res
				.status(400)
				.send(
					'Assistants API 的文件必须是 .doc, .docx, .pdf, .jpg, .jpeg, 或 .png 格式。'
				)
		}
		const fileSize = fs.statSync(req.file.path).size
		if (fileSize > 512 * 1024 * 1024) {
			fs.unlinkSync(req.file.path) // 删除不符合要求的文件
			return res.status(400).send('Assistants API 的文件大小不能超过 512 MB。')
		}
	}

	if (purpose === 'vision') {
		const supportedImageFormats = ['.jpg', '.jpeg', '.png']
		if (!supportedImageFormats.includes(fileExtension)) {
			fs.unlinkSync(req.file.path) // 删除不符合要求的文件
			return res.status(400).send('Vision API 仅支持 .jpg, .jpeg, .png 文件。')
		}
	}

	try {
		if (purpose === 'fine-tune' || purpose === 'search') {
			// 使用 OpenAI Files API
			const filePath = path.resolve(req.file.path)
			const fileStream = fs.createReadStream(filePath)
			const fileResponse = await openai.createFile(fileStream, purpose)

			// 清理上传的文件
			fs.unlinkSync(req.file.path)

			// 将 OpenAI 文件的创建结果发送回客户端
			res.json({
				id: fileResponse.data.id,
				filename: fileResponse.data.filename,
				mimetype: req.file.mimetype,
				bytes: fileResponse.data.bytes,
				created_at: fileResponse.data.created_at,
				purpose: fileResponse.data.purpose,
			})
		} else if (purpose === 'assistants' || purpose === 'vision') {
			// 生成文件的访问 URL
			const fileUrl = `https://${req.get('host')}/uploads/${req.file.filename}`

			// 获取文件大小
			const stats = fs.statSync(req.file.path)
			const fileSizeInBytes = stats.size

			// 生成唯一的文件 ID（例如，使用 UUID）
			const { v4: uuidv4 } = await import('uuid')
			const fileId = uuidv4()

			// 获取当前时间戳
			const createdAt = Math.floor(Date.now() / 1000) // Unix 时间戳

			// 返回完整的文件元数据
			res.json({
				id: fileId,
				object: 'file',
				bytes: fileSizeInBytes,
				created_at: createdAt,
				filename: req.file.originalname,
				purpose: purpose,
				url: fileUrl, // 仅在需要时包含 URL
			})
		} else {
			// 不支持的用途
			fs.unlinkSync(req.file.path)
			return res.status(400).send(`不支持的用途: ${purpose}`)
		}
	} catch (error) {
		if (error.response) {
			// OpenAI API 返回错误响应
			console.error(
				'OpenAI API Error:',
				error.response.status,
				error.response.data
			)
			res
				.status(error.response.status)
				.send({ error: error.response.data.error.message })
		} else {
			// 其他错误
			console.error('Error creating file with OpenAI API:', error.message)
			res.status(500).send({
				error:
					error.message || 'Error creating file for the specified purpose.',
			})
		}
	}
})

// 辅助函数：转换 PPTX 文件为 TXT
function convertPptxToText(filePath) {
	return new Promise((resolve, reject) => {
		const outputFilePath = path.join(
			'/tmp',
			`${path.basename(filePath, '.pptx')}.txt`
		)
		exec(`unoconv -f txt -o ${outputFilePath} ${filePath}`, (error) => {
			if (error) {
				return reject(error)
			}
			fs.readFile(outputFilePath, 'utf8', (err, data) => {
				if (err) return reject(err)
				resolve(data)
				// 删除临时文件
				unlink(outputFilePath).catch(console.error)
			})
		})
	})
}

// 新增路由：解析文件
app.post('/parse-file', async (req, res) => {
	const { fileId } = req.body

	if (!fileId) {
		return res.status(400).send('必须提供 fileId 参数。')
	}

	try {
		// 检索文件信息并获取下载 URL
		const fileInfo = await openai.files.retrieve(fileId)
		const downloadUrl = fileInfo.data.url

		// 下载文件并保存到临时目录
		const tempFilePath = path.join(
			uploadDir,
			`${fileId}${path.extname(fileInfo.data.filename)}`
		)
		const response = await fetch(downloadUrl, {
			headers: { Authorization: `Bearer ${API_KEY}` },
		})
		if (!response.ok) throw new Error('无法下载文件。')

		const fileStream = fs.createWriteStream(tempFilePath)
		await new Promise((resolve, reject) => {
			response.body.pipe(fileStream)
			response.body.on('error', reject)
			fileStream.on('finish', resolve)
		})

		// 根据文件类型解析内容
		const fileExtension = path.extname(fileInfo.data.filename).toLowerCase()
		let parsedData = ''

		if (fileExtension === '.pdf') {
			const pdfData = fs.readFileSync(tempFilePath)
			const pdfText = await pdfParse(pdfData)
			parsedData = pdfText.text
		} else if (['.doc', '.docx'].includes(fileExtension)) {
			const docxData = fs.readFileSync(tempFilePath)
			const docxResult = await mammoth.extractRawText({ buffer: docxData })
			parsedData = docxResult.value
		} else if (fileExtension === '.xlsx') {
			const workbook = xlsx.readFile(tempFilePath)
			const jsonData = {}
			workbook.SheetNames.forEach((sheetName) => {
				const sheet = workbook.Sheets[sheetName]
				const sheetData = xlsx.utils.sheet_to_json(sheet, { defval: '' })
				jsonData[sheetName] = sheetData
			})
			parsedData = JSON.stringify(jsonData)
		} else if (fileExtension === '.pptx') {
			// 使用 unoconv 解析 .pptx 文件
			parsedData = await convertPptxToText(tempFilePath)
		} else {
			await unlink(tempFilePath)
			return res.status(400).send('不支持的文件类型用于解析。')
		}

		// 清理临时文件
		await unlink(tempFilePath)

		// 返回解析后的数据
		res.json({ success: true, data: parsedData })
	} catch (error) {
		console.error(error)
		res.status(500).send({ error: error.message || 'Error parsing the file.' })
	}
})

// 新增路由以列出所有OpenAI文件
app.get('/list-files', async (req, res) => {
	try {
		const filesResponse = await openai.files.list()
		const files = []

		// 检查是否返回的是异步迭代器
		if (Symbol.asyncIterator in filesResponse) {
			for await (const file of filesResponse) {
				files.push(file)
			}
		} else {
			// 直接使用返回的数据
			files.push(...filesResponse.data)
		}

		// 将文件列表发送回客户端
		res.json(files)
	} catch (error) {
		console.error(error)
		res.status(500).send({ error: 'Error listing files.' })
	}
})

app.get('/retrieve-file/:fileId', async (req, res) => {
	const fileId = req.params.fileId

	try {
		const file = await openai.files.retrieve(fileId)
		res.json(file)
	} catch (error) {
		console.error(error)
		res.status(500).send({ error: 'Error retrieving file.' })
	}
})

// 设置路由以删除指定的OpenAI文件
app.delete('/delete-file/:fileId', async (req, res) => {
	const fileId = req.params.fileId // 从URL参数中获取文件ID

	try {
		// 调用OpenAI的API以删除文件
		const file = await openai.files.delete(fileId)
		res.json({
			success: true,
			message: 'File deleted successfully',
			file: file,
		})
	} catch (error) {
		console.error(error)
		res.status(500).send({ error: 'Error deleting file.' })
	}
})

app.get('/retrieve-file-content/:fileId', async (req, res) => {
	const fileId = req.params.fileId // 从URL参数中获取文件ID

	try {
		// 使用OpenAI SDK检索文件内容
		const fileContent = await openai.files.retrieveContent(fileId)
		res.send(fileContent)
	} catch (error) {
		console.error(error)
		res.status(500).send({ error: 'Error retrieving file content.' })
	}
})

// 轮询 fal 任务状态
const pollFalTaskStatus = async (modelPath, taskId, interval, maxAttempts) => {
	let attempts = 0

	while (attempts < maxAttempts) {
		await new Promise((resolve) => setTimeout(resolve, interval))
		attempts += 1

		try {
			// 使用 fal 客户端查询任务状态
			const status = await fal.queue.status(modelPath, {
				requestId: taskId,
				logs: true,
			})

			console.log(`任务ID: ${taskId} 状态:`, status)

			if (status.status === 'SUCCEEDED' || status.status === 'COMPLETED') {
				// 检查 `videoUrls` 是否存在
				if (status.videoUrls && Array.isArray(status.videoUrls)) {
					console.log(`任务 ${taskId} 已成功完成。输出:`, status.videoUrls)
					return status.videoUrls
				} else if (status.video && status.video.url) {
					console.log(`任务 ${taskId} 已成功完成。输出:`, [status.video.url])
					return [status.video.url]
				} else if (status.response_url) {
					console.log(
						`任务 ${taskId} 已成功完成。使用 response_url: ${status.response_url}`
					)
					return [status.response_url]
				} else {
					console.warn(`任务 ${taskId} 成功完成，但未找到 videoUrls。`)
					return []
				}
			} else if (status.status === 'FAILED') {
				console.error(`任务 ${taskId} 失败。详细信息:`, status)
				throw new Error(`视频生成失败: ${status.error || '未知错误。'}`)
			}
		} catch (error) {
			console.error(`轮询 fal 任务ID: ${taskId} 时出错:`, error)
			throw error
		}
	}

	throw new Error('视频生成超时。')
}

// 轮询 RunwayML 任务状态
const pollRunwayTaskStatus = async (
	taskId,
	interval = 10000,
	maxAttempts = 30
) => {
	let attempts = 0
	let lastStatus = null

	while (attempts < maxAttempts) {
		attempts += 1

		try {
			// 获取任务状态
			const task = await runwayClient.tasks.retrieve(taskId)
			console.log(
				`任务ID: ${taskId} 状态: ${task.status} (尝试次数: ${attempts}/${maxAttempts})`
			)

			// 缓存任务状态
			lastStatus = task.status

			if (task.status === 'SUCCEEDED') {
				console.log(`任务 ${taskId} 已成功完成。输出:`, task.output)
				return task.output // 假设 runwayClient 返回 output 数组
			} else if (task.status === 'FAILED') {
				console.error(`任务 ${taskId} 失败。详细信息:`, task)
				throw new Error(`视频生成失败: ${task.failure || '未知错误。'}`)
			}

			// 动态调整轮询间隔（指数退避）
			const dynamicInterval = interval * Math.pow(1.5, attempts - 1)
			console.log(`下次轮询间隔: ${dynamicInterval} 毫秒`)
			await new Promise((resolve) => setTimeout(resolve, dynamicInterval))
		} catch (error) {
			console.error(`轮询 RunwayML 任务ID: ${taskId} 时出错:`, error)

			// 如果任务状态已知，返回更具体的错误信息
			if (lastStatus === 'FAILED') {
				throw new Error(`视频生成失败: ${error.message}`)
			}

			// 如果达到最大尝试次数，抛出超时错误
			if (attempts >= maxAttempts) {
				throw new Error(`视频生成超时: ${error.message}`)
			}

			// 继续重试
			console.warn(
				`任务 ${taskId} 轮询失败，重试中 (${attempts}/${maxAttempts})...`
			)
		}
	}

	throw new Error('视频生成超时。')
}

// 创建视频生成任务接口，添加任务到队列
app.post('/runway/video-creator', async (req, res) => {
	const { promptText, promptImage } = req.body

	if (!promptText || !promptImage) {
		return res
			.status(400)
			.json({ error: 'promptText 和 promptImage 是必填项。' })
	}

	try {
		console.log('创建视频生成任务的参数:', {
			model: 'gen3a_turbo',
			promptImage,
			promptText,
			duration: 10,
			watermark: false,
			ratio: '1280:768',
		})

		// 提交视频生成任务
		const task = await runwayClient.imageToVideo.create({
			model: 'gen3a_turbo',
			promptImage: promptImage, // 支持 URL 或 base64 数据 URI
			promptText: promptText,
			duration: 10, // 可选参数，根据需要调整
			watermark: false, // 可选参数
			ratio: '1280:768', // 可选参数
		})

		console.log('RunwayML Task Created:', task)

		// 初始化任务状态并保存到 Redis
		await redisClient.hSet(`task:${task.id}`, {
			status: 'RUNNING',
			videoUrls: JSON.stringify([]),
		})

		// 添加任务到 Bull 队列
		await videoQueue.add({ taskId: task.id })

		// 返回任务 ID
		res.status(202).json({ taskId: task.id })
	} catch (error) {
		console.error('视频生成失败:', error)
		res
			.status(500)
			.json({ error: error.message || '视频生成失败，请稍后再试。' })
	}
})

// 查询任务状态接口
app.get('/runway/task-status/:id', async (req, res) => {
	const taskId = req.params.id

	try {
		const task = await redisClient.hGetAll(`task:${taskId}`)
		if (!task || Object.keys(task).length === 0) {
			return res.status(404).json({ error: '任务未找到。' })
		}

		res.status(200).json({
			status: task.status,
			videoUrls: task.videoUrls ? JSON.parse(task.videoUrls) : [],
		})
	} catch (error) {
		console.error('查询任务状态时出错:', error)
		res.status(500).json({ error: '查询任务状态时出错。' })
	}
})

// 删除任务的接口
app.delete('/runway/tasks/:id', async (req, res) => {
	const taskId = req.params.id

	try {
		await runwayClient.tasks.delete(taskId)
		console.log(`任务已删除，ID: ${taskId}`)
		// 从 Redis 中移除任务
		await redisClient.del(`task:${taskId}`)
		res.status(204).send() // 204 No Content
	} catch (error) {
		console.error('删除任务失败:', error)
		res
			.status(500)
			.json({ error: error.message || '删除任务失败，请稍后再试。' })
	}
})

// 定义下载视频的辅助函数
const downloadVideo = async (url, savePath) => {
	const writer = fs.createWriteStream(savePath)

	const response = await axios({
		url,
		method: 'GET',
		responseType: 'stream',
	})

	response.data.pipe(writer)

	return new Promise((resolve, reject) => {
		writer.on('finish', resolve)
		writer.on('error', reject)
	})
}

// 处理 Bull 队列中的任务
videoQueue.process(async (job) => {
	const { taskId, modelPath, videoUrl } = job.data

	console.log(
		`Processing job: taskId=${taskId}, modelPath=${modelPath}, videoUrl=${videoUrl}`
	)

	try {
		let outputUrls = []

		if (videoUrl) {
			// 如果 videoUrl 存在，直接使用它
			outputUrls = [videoUrl]
			console.log(`使用订阅结果中的 videoUrl: ${videoUrl}`)
		} else if (modelPath && modelPath.startsWith('fal-ai/')) {
			// 如果 modelPath 以 'fal-ai/' 开头，使用 FAL 客户端轮询任务状态
			outputUrls = await pollFalTaskStatus(modelPath, taskId)
		} else {
			// 否则，使用 RunwayML 客户端轮询任务状态
			outputUrls = await pollRunwayTaskStatus(taskId)
		}

		if (!outputUrls || outputUrls.length === 0) {
			throw new Error('未能获取视频 URL。')
		}

		// 下载并存储每个视频文件
		const downloadPromises = outputUrls.map(async (url, index) => {
			const parsedUrl = new URL(url)
			const extension = path.extname(parsedUrl.pathname) || '.mp4' // 获取扩展名，默认 .mp4
			const fileName = `video_${taskId}_${index + 1}${extension}`
			const savePath = path.join(VIDEOS_DIR, fileName)

			await downloadVideo(url, savePath)
			return fileName // 返回文件名以构建 URL
		})

		const savedFiles = await Promise.all(downloadPromises)

		// 构建公共 URL
		const savedUrls = savedFiles.map(
			(fileName) => `https://proxy.star-ai.net/videos/${fileName}`
		)

		// 更新任务状态并保存到 Redis
		await redisClient.hSet(`task:${taskId}`, {
			status: 'SUCCEEDED',
			videoUrls: JSON.stringify(savedUrls),
		})

		console.log(`任务完成，视频已下载至: ${savedFiles}`)
		console.log(`视频公共URL: ${savedUrls}`)

		// 通过 Socket.IO 通知前端
		io.to(taskId).emit('taskCompleted', { videoUrls: savedUrls })
	} catch (error) {
		// 更新任务状态为 FAILED
		await redisClient.hSet(`task:${taskId}`, {
			status: 'FAILED',
			error: error.message || '视频生成失败。',
		})
		console.error(`任务失败，ID: ${taskId}`, error)
		// 通过 Socket.IO 通知前端
		io.to(taskId).emit('taskFailed', {
			error: error.message || '视频生成失败。',
		})
	}
})

// Socket.IO 连接处理
io.on('connection', (socket) => {
	console.log('一个用户已连接')

	// 处理加入任务房间
	socket.on('joinTask', async (taskId) => {
		socket.join(taskId)
		console.log(`用户加入任务房间: ${taskId}`)

		try {
			// 从 Redis 获取任务状态
			const taskData = await redisClient.hGetAll(`task:${taskId}`)

			if (taskData.status === 'SUCCEEDED' || taskData.status === 'COMPLETED') {
				const videoUrls = JSON.parse(taskData.videoUrls || '[]')
				socket.emit('taskCompleted', { videoUrls })
				console.log(`任务 ${taskId} 已完成，已通知前端`)
			} else if (taskData.status === 'FAILED') {
				const error = taskData.error || '视频生成失败。'
				socket.emit('taskFailed', { error })
				console.log(`任务 ${taskId} 已失败，已通知前端`)
			} else if (taskData.status === 'RUNNING') {
				// 任务仍在运行，可以选择发送当前状态或不发送
				console.log(`任务 ${taskId} 仍在运行`)
			} else {
				// 未知状态
				console.warn(`任务 ${taskId} 处于未知状态: ${taskData.status}`)
			}
		} catch (error) {
			console.error(`获取任务 ${taskId} 状态时出错:`, error)
		}
	})

	// 处理断开连接
	socket.on('disconnect', () => {
		console.log('用户已断开连接')
	})
})

/**
 * 路由：订阅任务
 * POST /fal/image-to-video/subscribe
 * 请求体：
 * {
 *   "model": "fal-ai/ltx-video/image-to-video",
 *   "prompt": "详细的描述...",
 *   "image_url": "https://example.com/image.jpg"
 * }
 */
app.post('/fal/image-to-video/subscribe', async (req, res) => {
	const { model, prompt, image_url } = req.body

	// 验证必填参数
	if (!model || !prompt || !image_url) {
		return res
			.status(400)
			.json({ error: 'model, prompt 和 image_url 是必填项。' })
	}

	// 验证模型格式
	const modelPattern = /^fal-ai\/.+\/image-to-video$/
	if (!modelPattern.test(model)) {
		return res.status(400).json({ error: 'model 参数格式不正确。' })
	}

	try {
		const result = await fal.subscribe(model, {
			input: {
				prompt,
				image_url,
			},
			logs: true,
			onQueueUpdate: (update) => {
				if (update.status === 'IN_PROGRESS') {
					update.logs.map((log) => log.message).forEach(console.log)
				}
			},
		})

		console.log('Subscribe Result:', result.data)
		console.log('Request ID:', result.requestId)

		// 保存任务状态到 Redis
		await redisClient.hSet(`task:${result.requestId}`, {
			status: 'RUNNING',
			videoUrls: JSON.stringify([]),
		})

		// 提取 video URL
		const videoUrl = result.data.video.url

		// 添加任务到 Bull 队列，并包含模型路径和 videoUrl
		console.log(
			`Adding job to queue: taskId=${result.requestId}, modelPath=${model}, videoUrl=${videoUrl}`
		)
		await videoQueue.add(
			{ taskId: result.requestId, modelPath: model, videoUrl },
			{
				attempts: 3, // 重试次数
				backoff: 5000, // 重试间隔（毫秒）
			}
		)

		res.status(200).json({
			data: result.data,
			requestId: result.requestId,
		})
	} catch (error) {
		console.error('订阅任务失败:', error)

		// 如果错误包含详细信息，返回这些信息
		if (error.body && error.body.detail) {
			return res
				.status(422)
				.json({ error: '订阅任务失败：' + JSON.stringify(error.body.detail) })
		}

		res.status(500).json({ error: '订阅任务失败，请稍后再试。' })
	}
})

/**
 * 路由：提交请求
 * POST /fal/image-to-video/submit
 * 请求体：
 * {
 *   "model": "fal-ai/ltx-video/image-to-video",
 *   "prompt": "详细的描述...",
 *   "image_url": "https://example.com/image.jpg",
 *   "webhookUrl": "https://your.webhook.url/for/results" // 可选
 * }
 */
app.post('/fal/image-to-video/submit', async (req, res) => {
	const { model, prompt, image_url, webhookUrl } = req.body

	// 验证必填参数
	if (!model || !prompt || !image_url) {
		return res
			.status(400)
			.json({ error: 'model, prompt 和 image_url 是必填项。' })
	}

	// 验证模型格式
	const modelPattern = /^fal-ai\/.+\/image-to-video$/
	if (!modelPattern.test(model)) {
		return res.status(400).json({ error: 'model 参数格式不正确。' })
	}

	try {
		const { request_id } = await fal.queue.submit(model, {
			input: {
				prompt,
				image_url,
			},
			webhookUrl: webhookUrl || null,
		})

		res.status(200).json({ request_id })
	} catch (error) {
		console.error('提交请求失败:', error)
		res.status(500).json({ error: '提交请求失败，请稍后再试。' })
	}
})

/**
 * 路由：获取请求状态
 * POST /fal/image-to-video/status
 * 请求体：
 * {
 *   "model": "fal-ai/ltx-video/image-to-video",
 *   "requestId": "764cabcf-b745-4b3e-ae38-1200304cf45b",
 *   "logs": true // 可选
 * }
 */
app.post('/fal/image-to-video/status', async (req, res) => {
	const { model, requestId, logs } = req.body

	// 验证必填参数
	if (!model || !requestId) {
		return res.status(400).json({ error: 'model 和 requestId 是必填项。' })
	}

	// 验证模型格式
	const modelPattern = /^fal-ai\/.+\/image-to-video$/
	if (!modelPattern.test(model)) {
		return res.status(400).json({ error: 'model 参数格式不正确。' })
	}

	try {
		const status = await fal.queue.status(model, {
			requestId,
			logs: logs === true,
		})

		res.status(200).json(status)
	} catch (error) {
		console.error('获取请求状态失败:', error)
		res.status(500).json({ error: '获取请求状态失败，请稍后再试。' })
	}
})

/**
 * 路由：获取请求结果
 * POST /fal/image-to-video/result
 * 请求体：
 * {
 *   "model": "fal-ai/ltx-video/image-to-video",
 *   "requestId": "764cabcf-b745-4b3e-ae38-1200304cf45b"
 * }
 */
app.post('/fal/image-to-video/result', async (req, res) => {
	const { model, requestId } = req.body

	// 验证必填参数
	if (!model || !requestId) {
		return res.status(400).json({ error: 'model 和 requestId 是必填项。' })
	}

	// 验证模型格式
	const modelPattern = /^fal-ai\/.+\/image-to-video$/
	if (!modelPattern.test(model)) {
		return res.status(400).json({ error: 'model 参数格式不正确。' })
	}

	try {
		const result = await fal.queue.result(model, {
			requestId,
		})

		console.log(result.data)
		console.log(result.requestId)

		// 假设 result.data 包含视频 URL，您可以根据需要进一步处理
		res.status(200).json({
			data: result.data,
			requestId: result.requestId,
		})
	} catch (error) {
		console.error('获取请求结果失败:', error)
		res.status(500).json({ error: '获取请求结果失败，请稍后再试。' })
	}
})

app.post('/fal/image-to-video/cancel', async (req, res) => {
	const { model, requestId } = req.body

	// 验证必填参数
	if (!requestId) {
		return res.status(400).json({ error: 'requestId 是必填项。' })
	}

	try {
		let job

		if (requestId) {
			// 假设 Job ID 就是 requestId
			job = await videoQueue.getJob(requestId)
		}

		if (job) {
			const state = await job.getState()

			if (state === 'completed' || state === 'failed' || state === 'canceled') {
				return res.status(400).json({ error: '任务已完成或无法取消。' })
			}

			// 移除队列中的 Job（适用于等待中的任务）
			await job.remove()

			// 更新 Redis 中任务状态为 'canceled'
			await redisClient.hSet(`task:${requestId}`, 'status', 'canceled')

			console.log(`任务已取消：${requestId}`)
			return res.status(200).json({ message: '任务已取消。' })
		} else {
			// 如果 Job 不在等待队列中，检查是否在活跃队列中
			const activeJobs = await videoQueue.getActive()
			const jobToCancel = activeJobs.find((j) => j.id === requestId)

			if (jobToCancel) {
				// 标记 Job 为失败，并传递取消原因
				await jobToCancel.moveToFailed({ message: '任务已取消。' }, true)

				// 更新 Redis 中任务状态为 'canceled'
				await redisClient.hSet(`task:${requestId}`, 'status', 'canceled')

				console.log(`正在处理中的任务已取消：${requestId}`)
				return res.status(200).json({ message: '正在处理中的任务已取消。' })
			} else {
				// 任务未找到
				return res.status(404).json({ error: '任务未找到。' })
			}
		}
	} catch (error) {
		console.error('取消任务失败:', error)
		return res.status(500).json({ error: '服务器内部错误，无法取消任务。' })
	}
})

app.post('/flux-pro-ultra', async (req, res) => {
	const { prompt, size, num_images, aspect_ratio, output_format, seed, raw } =
		req.body

	try {
		// 提交请求到 FLUX1.1 [pro] ultra API
		const fluxResult = await fal.subscribe('fal-ai/flux-pro/v1.1-ultra', {
			input: {
				prompt: prompt,
				num_images: num_images || 1,
				enable_safety_checker: false, // 默认启用
				safety_tolerance: '2', // 默认值
				output_format: output_format || 'jpeg',
				aspect_ratio: aspect_ratio || '16:9',
				// seed: seed, // 移除 seed 参数
				// raw: raw || false, // 移除 raw 参数
			},
			logs: true,
			onQueueUpdate: (update) => {
				if (update.status === 'IN_PROGRESS') {
					// 打印日志到服务器控制台
					update.logs.map((log) => log.message).forEach(console.log)
				}
			},
		})

		// 假设 fluxResult.data 包含生成的图像信息
		res.json(fluxResult.data)
	} catch (error) {
		console.error('Error generating image:', error)
		res.status(500).json({ error: '图片生成失败' })
	}
})

// 图片生成路由
app.post('/image', async (req, res) => {
	const {
		model = 'dall-e-2',
		prompt,
		n = 1,
		size = '1024x1024',
		quality,
		style,
		response_format = 'url',
	} = req.body

	// 参数验证
	if (!prompt) {
		return res.status(400).json({ error: '提示词是必填项。' })
	}

	if (model === 'dall-e-3' && n !== 1) {
		return res.status(400).json({ error: 'dall-e-3仅支持生成1张图片。' })
	}

	// Size validation
	const sizeOptions =
		model === 'dall-e-2'
			? ['256x256', '512x512', '1024x1024']
			: ['1024x1024', '1792x1024', '1024x1792']
	if (!sizeOptions.includes(size)) {
		return res
			.status(400)
			.json({ error: `尺寸必须是以下选项之一：${sizeOptions.join(', ')}。` })
	}

	// Quality and Style validation for dall-e-3
	if (model === 'dall-e-3') {
		const qualityOptions = ['standard', 'hd']
		if (quality && !qualityOptions.includes(quality)) {
			return res.status(400).json({
				error: `质量必须是以下选项之一：${qualityOptions.join(', ')}。`,
			})
		}

		const styleOptions = ['vivid', 'natural']
		if (style && !styleOptions.includes(style)) {
			return res
				.status(400)
				.json({ error: `风格必须是以下选项之一：${styleOptions.join(', ')}。` })
		}
	}

	// Response format validation
	const responseFormatOptions = ['url', 'b64_json']
	if (response_format && !responseFormatOptions.includes(response_format)) {
		return res.status(400).json({
			error: `响应格式必须是以下选项之一：${responseFormatOptions.join(
				', '
			)}。`,
		})
	}

	try {
		const requestParams = {
			prompt,
			n,
			size,
			response_format,
		}

		if (model !== 'dall-e-2') {
			requestParams.model = model
			if (quality) requestParams.quality = quality
			if (style) requestParams.style = style
		}

		const openaiResponse = await openai.images.generate(requestParams)
		res.json(openaiResponse.data) // 确保返回的是包含 "data" 字段的对象
	} catch (error) {
		console.error(
			'Error generating image:',
			error.response?.data || error.message
		)
		res.status(500).json({ error: '图片生成失败，请稍后再试。' })
	}
})

// 图像编辑路由
app.post(
	'/edit-image',
	upload.fields([
		{ name: 'image', maxCount: 1 },
		{ name: 'mask', maxCount: 1 },
	]),
	async (req, res) => {
		if (!req.files || !req.files.image || !req.files.mask) {
			return res.status(400).send('需要上传图片和遮罩文件。')
		}

		const { prompt } = req.body

		if (!prompt) {
			return res.status(400).send('编辑提示词是必填项。')
		}

		try {
			const imagePath = path.resolve(req.files.image[0].path)
			const maskPath = path.resolve(req.files.mask[0].path)

			const imageResponse = await openai.images.edit({
				image: fs.createReadStream(imagePath),
				mask: fs.createReadStream(maskPath),
				prompt,
			})

			// 清理上传的文件
			fs.unlinkSync(imagePath)
			fs.unlinkSync(maskPath)

			res.json(imageResponse.data)
		} catch (error) {
			console.error(
				'Error editing image:',
				error.response?.data || error.message
			)
			// 清理上传的文件
			if (req.files.image) fs.unlinkSync(path.resolve(req.files.image[0].path))
			if (req.files.mask) fs.unlinkSync(path.resolve(req.files.mask[0].path))
			res.status(500).send({ error: '图像编辑失败。' })
		}
	}
)

// 图像变体创建路由
app.post('/create-image-variation', async (req, res) => {
	const { imageUrl, size = '1024x1024', n = 1 } = req.body

	// 验证参数
	if (!imageUrl) {
		return res.status(400).json({ error: '需要提供图片 URL。' })
	}

	// 尺寸验证（根据 OpenAI 的要求修改尺寸选项）
	const sizeOptions = ['256x256', '512x512', '1024x1024']
	if (!sizeOptions.includes(size)) {
		return res
			.status(400)
			.json({ error: `尺寸必须是以下选项之一：${sizeOptions.join(', ')}。` })
	}

	// 生成数量验证（dall-e-2 支持 1 到 10）
	if (typeof n !== 'number' || n < 1 || n > 10) {
		return res.status(400).json({ error: '数量必须是 1 到 10 之间的整数。' })
	}

	try {
		// 下载图片
		const response = await axios.get(imageUrl, { responseType: 'arraybuffer' })
		const buffer = Buffer.from(response.data, 'binary')

		// 检查文件大小（小于 4MB）
		const fileSizeInMB = buffer.length / (1024 * 1024)
		if (fileSizeInMB > 4) {
			throw new Error('图片大小超过 4MB。')
		}

		// 使用 sharp 检查图片格式并调整为正方形
		let image = sharp(buffer)
		const metadata = await image.metadata()

		if (metadata.format !== 'png') {
			throw new Error('图片格式必须为 PNG。')
		}

		if (metadata.width !== metadata.height) {
			// 调整图片为正方形（裁剪中心区域）
			const minDimension = Math.min(metadata.width, metadata.height)
			image = image.extract({
				left: Math.floor((metadata.width - minDimension) / 2),
				top: Math.floor((metadata.height - minDimension) / 2),
				width: minDimension,
				height: minDimension,
			})
		}

		// 将调整后的图片转换为 PNG 格式
		image = image.png()

		// 获取调整后的图片 Buffer
		const processedBuffer = await image.toBuffer()

		// 保存调整后的图片到临时文件
		const tempFilePath = path.join(
			__dirname,
			'uploads',
			`temp-${Date.now()}.png`
		)

		// 确保上传目录存在
		fs.mkdirSync(path.dirname(tempFilePath), { recursive: true })

		// 保存图片
		await fs.promises.writeFile(tempFilePath, processedBuffer)

		// 调用 OpenAI API 创建图像变体
		const imageResponse = await openai.images.createVariation({
			image: fs.createReadStream(tempFilePath),
			n, // 数量
			size,
			response_format: 'url', // 默认为 'url'
			// model 参数已移除
		})

		// 清理临时文件
		fs.unlinkSync(tempFilePath)

		// 打印 OpenAI API 响应日志
		console.log(
			'OpenAI API response:',
			JSON.stringify(imageResponse.data, null, 2)
		)

		// 检查并返回生成的图像 URL
		if (
			Array.isArray(imageResponse.data) &&
			imageResponse.data.length > 0 &&
			imageResponse.data[0].url
		) {
			const urls = imageResponse.data.map((img) => img.url)
			res.json({ urls })
		} else if (
			imageResponse.data &&
			imageResponse.data.data &&
			imageResponse.data.data.length > 0 &&
			imageResponse.data.data[0].url
		) {
			// 兼容旧版本 SDK 的响应结构
			const urls = imageResponse.data.data.map((img) => img.url)
			res.json({ urls })
		} else {
			throw new Error('未能获取图像变体的 URL。')
		}
	} catch (error) {
		console.error(
			'Error creating image variation:',
			error.response?.data || error.message
		)
		res.status(500).json({ error: error.message || '创建图像变体失败。' })
	}
})

// app.post('/image', async (req, res) => {
// 	try {
// 		const response = await openai.images.generate({
// 			model: req.body.model,
// 			prompt: req.body.message,
// 			n: req.body.number,
// 			size: req.body.size,
// 		})
// 		const data = await response.data
// 		res.send(data)
// 	} catch (error) {
// 		console.error(error)
// 	}
// })

// 设置一个路由来处理图像编辑请求
// app.post(
// 	'/edit-image',
// 	upload.fields([
// 		{ name: 'image', maxCount: 1 },
// 		{ name: 'mask', maxCount: 1 },
// 	]),
// 	async (req, res) => {
// 		if (!req.files || !req.files.image || !req.files.mask) {
// 			return res.status(400).send('Image and mask files are required.')
// 		}

// 		try {
// 			const imagePath = path.resolve(req.files.image[0].path)
// 			const maskPath = path.resolve(req.files.mask[0].path)
// 			const prompt = req.body.prompt // 获取文本提示

// 			const imageResponse = await openai.images.edit({
// 				image: fs.createReadStream(imagePath),
// 				mask: fs.createReadStream(maskPath),
// 				prompt: prompt,
// 			})

// 			// 清理上传的文件
// 			fs.unlinkSync(imagePath)
// 			fs.unlinkSync(maskPath)

// 			// 将编辑后的图像信息发送回客户端
// 			res.json(imageResponse.data)
// 		} catch (error) {
// 			console.error(error)
// 			// 清理上传的文件（在出现错误的情况下）
// 			if (req.files.image) fs.unlinkSync(path.resolve(req.files.image[0].path))
// 			if (req.files.mask) fs.unlinkSync(path.resolve(req.files.mask[0].path))
// 			res.status(500).send({ error: 'Error editing image.' })
// 		}
// 	}
// )

// 设置路由来处理图像变体创建请求
// app.post(
// 	'/create-image-variation',
// 	upload.single('image'),
// 	async (req, res) => {
// 		if (!req.file) {
// 			return res.status(400).send('Image file is required.')
// 		}

// 		try {
// 			const imagePath = path.resolve(req.file.path)

// 			const imageResponse = await openai.images.createVariation({
// 				image: fs.createReadStream(imagePath),
// 			})

// 			// 清理上传的文件
// 			fs.unlinkSync(imagePath)

// 			// 将创建的图像变体信息发送回客户端
// 			res.json(imageResponse.data)
// 		} catch (error) {
// 			console.error(error)
// 			// 清理上传的文件（在出现错误的情况下）
// 			fs.unlinkSync(imagePath)
// 			res.status(500).send({ error: 'Error creating image variation.' })
// 		}
// 	}
// )

app.get('/list-models', async (req, res) => {
	try {
		const modelsResponse = await openai.models.list()
		const models = []

		// 如果返回的数据是异步迭代器，则遍历它以收集模型
		if (Symbol.asyncIterator in modelsResponse) {
			for await (const model of modelsResponse) {
				models.push(model)
			}
		} else {
			// 如果数据直接以列表形式返回，则直接使用
			models.push(...modelsResponse.data)
		}

		// 将模型列表发送回客户端
		res.json(models)
	} catch (error) {
		console.error(error)
		res.status(500).send({ error: 'Error listing models.' })
	}
})

app.get('/retrieve-model/:modelId', async (req, res) => {
	const modelId = req.params.modelId // 从URL参数中获取模型ID

	try {
		const model = await openai.models.retrieve(modelId)
		res.json(model)
	} catch (error) {
		console.error(error)
		res.status(500).send({ error: 'Error retrieving model.' })
	}
})

// 设置一个路由来处理删除模型的请求
app.delete('/delete-model/:modelId', async (req, res) => {
	const modelId = req.params.modelId // 从URL参数中获取模型ID

	try {
		// 调用OpenAI的API以删除指定的模型
		const response = await openai.models.delete(modelId)
		res.json({
			success: true,
			message: 'Model deleted successfully',
			response: response,
		})
	} catch (error) {
		console.error(error)
		res.status(500).send({ error: 'Error deleting model.' })
	}
})

// 处理文本内容的审核请求
app.post('/moderate-content', async (req, res) => {
	const { input } = req.body

	if (!input) {
		return res.status(400).send('Input text is required.')
	}

	try {
		const moderation = await openai.moderations.create({ input: input })
		res.json(moderation.data)
	} catch (error) {
		console.error(error)
		res.status(500).send({ error: 'Error moderating content.' })
	}
})

// 添加一个新的POST路由来处理 /assistant 请求
app.post('/assistant', async (req, res) => {
	try {
		// 使用请求体中提供的信息来创建一个新的助手
		const assistantResponse = await openai.beta.assistants.create({
			instructions: req.body.instructions,
			name: req.body.name,
			tools: req.body.tools,
			model: req.body.model,
			// file_ids: req.body.file_ids,
		})

		// 调试日志
		console.log('Assistant Response:', assistantResponse)
		const data = assistantResponse.data
		console.log('Assistant Data:', data)

		// 从OpenAI API响应中获取数据并将其发送回客户端
		res.send(data)
	} catch (error) {
		console.error(
			'Error details:',
			error.response ? error.response.data : error.message
		)
		res.status(500).send({
			error: 'An error occurred while creating the assistant.',
			details: error.response ? error.response.data : error.message,
		})
	}
})

// app.post('/assistant', async (req, res) => {
// 	try {
// 		// 确保 tools 是数组
// 		const toolsArray = Array.isArray(req.body.tools)
// 			? req.body.tools
// 			: req.body.tools.split(',').map((tool) => tool.trim())

// 		// 使用请求体中提供的信息来创建一个新的助手，不包括 file_ids
// 		const assistantResponse = await openai.beta.assistants.create({
// 			instructions: req.body.instructions,
// 			name: req.body.name,
// 			tools: toolsArray, // 传递数组
// 			model: req.body.model,
// 			// 移除 file_ids，如果不需要传递
// 		})

// 		// 从OpenAI API响应中获取数据并将其发送回客户端
// 		const data = assistantResponse.data
// 		res.send(data)
// 	} catch (error) {
// 		console.error(
// 			'Error details:',
// 			error.response ? error.response.data : error.message
// 		)
// 		res.status(500).send({
// 			error: 'An error occurred while creating the assistant.',
// 			details: error.response ? error.response.data : error.message,
// 		})
// 	}
// })

app.post('/assistant-file', async (req, res) => {
	try {
		// 从请求体中获取助手ID和文件ID
		const assistantId = req.body.assistantId
		const fileId = req.body.fileId

		if (!assistantId || !fileId) {
			return res
				.status(400)
				.send({ error: 'Both assistantId and fileId are required.' })
		}

		// 调用OpenAI API来创建助手文件
		const assistantFileResponse = await openai.beta.assistants.files.create(
			assistantId,
			{
				file_id: fileId,
			}
		)

		// 从OpenAI API响应中获取数据并将其发送回客户端
		const data = assistantFileResponse.data
		res.send(data)
	} catch (error) {
		console.error(error)
		res.status(500).send('An error occurred while creating the assistant file.')
	}
})

app.get('/list-assistants', async (req, res) => {
	try {
		// 调用OpenAI API来列出助手
		const assistantsResponse = await openai.beta.assistants.list({
			order: req.query.order || 'desc', // 如果客户端提供了order参数，就使用它；否则默认为"desc"
			limit: req.query.limit || 20, // 如果客户端提供了limit参数，就使用它；否则默认为20
		})

		// 从OpenAI API响应中获取数据并将其发送回客户端
		const data = assistantsResponse.data
		res.send(data)
	} catch (error) {
		console.error(error)
		res.status(500).send('An error occurred while listing the assistants.')
	}
})

app.get('/assistant/:assistant_id', async (req, res) => {
	try {
		// Retrieve the assistant using the provided assistant_id
		const assistantId = req.params.assistant_id
		const assistantResponse = await openai.beta.assistants.retrieve(assistantId)

		// Send the assistant data back to the client
		res.send(assistantResponse)
	} catch (error) {
		console.error(error)
		res.status(500).send('An error occurred while retrieving the assistant.')
	}
})

app.get('/assistant-files/:assistantId', async (req, res) => {
	try {
		// 从URL参数中获取助手ID
		const { assistantId } = req.params

		if (!assistantId) {
			return res.status(400).send({ error: 'Assistant ID is required.' })
		}

		// 调用OpenAI API来列出与助手相关联的文件
		const assistantFilesResponse = await openai.beta.assistants.files.list(
			assistantId
		)

		// 从OpenAI API响应中获取数据并将其发送回客户端
		const data = assistantFilesResponse.data
		res.send(data)
	} catch (error) {
		console.error(error)
		res.status(500).send('An error occurred while listing the assistant files.')
	}
})

app.get('/retrieve-assistant-file/:assistantId/:fileId', async (req, res) => {
	try {
		// 从URL参数中提取助手ID和文件ID
		const { assistantId, fileId } = req.params

		if (!assistantId || !fileId) {
			return res
				.status(400)
				.send({ error: 'Assistant ID and File ID are required.' })
		}

		// 调用OpenAI API来检索与助手相关联的文件
		const assistantFileResponse = await openai.beta.assistants.files.retrieve(
			assistantId,
			fileId
		)

		// 从OpenAI API响应中获取数据并将其发送回客户端
		const data = assistantFileResponse.data
		res.send(data)
	} catch (error) {
		console.error(error)
		res
			.status(500)
			.send('An error occurred while retrieving the assistant file.')
	}
})

app.post('/update-assistant/:assistantId', async (req, res) => {
	try {
		// 从URL参数中提取助手ID
		const { assistantId } = req.params

		if (!assistantId) {
			return res.status(400).send({ error: 'Assistant ID is required.' })
		}

		// 使用请求体中提供的信息来更新助手
		const updateData = {
			instructions: req.body.instructions,
			name: req.body.name,
			tools: req.body.tools,
			model: req.body.model,
			file_ids: req.body.file_ids,
		}

		// 调用OpenAI API来更新助手
		const updatedAssistantResponse = await openai.beta.assistants.update(
			assistantId,
			updateData
		)

		// 从OpenAI API响应中获取数据并将其发送回客户端
		const data = updatedAssistantResponse.data
		res.send(data)
	} catch (error) {
		console.error(error)
		res.status(500).send('An error occurred while updating the assistant.')
	}
})

app.delete('/delete-assistant/:assistantId', async (req, res) => {
	try {
		// 从URL参数中提取助手ID
		const { assistantId } = req.params

		if (!assistantId) {
			return res.status(400).send({ error: 'Assistant ID is required.' })
		}

		// 调用OpenAI API来删除助手
		const deleteResponse = await openai.beta.assistants.del(assistantId)

		// 如果API响应成功，返回成功信息给客户端
		res.send({ message: 'Assistant deleted successfully.' })
	} catch (error) {
		console.error(error)
		res.status(500).send('An error occurred while deleting the assistant.')
	}
})

app.delete('/delete-assistant-file/:assistantId/:fileId', async (req, res) => {
	try {
		// 从URL参数中提取助手ID和文件ID
		const { assistantId, fileId } = req.params

		if (!assistantId || !fileId) {
			return res
				.status(400)
				.send({ error: 'Assistant ID and File ID are required.' })
		}

		// 调用OpenAI API来删除与助手相关联的文件
		const deleteResponse = await openai.beta.assistants.files.del(
			assistantId,
			fileId
		)

		// 如果API响应成功，返回成功信息给客户端
		res.send({ message: 'Assistant file deleted successfully.' })
	} catch (error) {
		console.error(error)
		res.status(500).send('An error occurred while deleting the assistant file.')
	}
})

app.post('/create-thread-message/:threadId', async (req, res) => {
	const { threadId } = req.params
	const { role, content } = req.body

	if (!role || !content) {
		return res.status(400).send('Role and content are required.')
	}

	try {
		const threadMessages = await openai.beta.threads.messages.create(threadId, {
			role,
			content,
		})
		res.json(threadMessages.data)
	} catch (error) {
		console.error(error)
		res.status(500).send({ error: 'Error creating thread message.' })
	}
})

app.get('/list-thread-messages/:threadId', async (req, res) => {
	const { threadId } = req.params

	try {
		const threadMessages = await openai.beta.threads.messages.list(threadId)
		res.json(threadMessages.data)
	} catch (error) {
		console.error(error)
		res.status(500).send({ error: 'Error listing thread messages.' })
	}
})

app.get('/list-message-files/:threadId/:messageId', async (req, res) => {
	const { threadId, messageId } = req.params

	try {
		const messageFiles = await openai.beta.threads.messages.files.list(
			threadId,
			messageId
		)
		res.json(messageFiles.data)
	} catch (error) {
		console.error(error)
		res.status(500).send({ error: 'Error listing message files.' })
	}
})

app.get('/retrieve-message/:threadId/:messageId', async (req, res) => {
	const { threadId, messageId } = req.params

	try {
		const message = await openai.beta.threads.messages.retrieve(
			threadId,
			messageId
		)
		res.json(message.data)
	} catch (error) {
		console.error(error)
		res.status(500).send({ error: 'Error retrieving message.' })
	}
})

app.get(
	'/retrieve-message-file/:threadId/:messageId/:fileId',
	async (req, res) => {
		const { threadId, messageId, fileId } = req.params

		try {
			const messageFile = await openai.beta.threads.messages.files.retrieve(
				threadId,
				messageId,
				fileId
			)
			res.json(messageFile.data)
		} catch (error) {
			console.error(error)
			res.status(500).send({ error: 'Error retrieving message file.' })
		}
	}
)

app.put('/update-message/:threadId/:messageId', async (req, res) => {
	const { threadId, messageId } = req.params
	const { metadata } = req.body

	if (!metadata) {
		return res.status(400).send('Metadata for update is required.')
	}

	try {
		const updatedMessage = await openai.beta.threads.messages.update(
			threadId,
			messageId,
			{
				metadata,
			}
		)
		res.json(updatedMessage.data)
	} catch (error) {
		console.error(error)
		res.status(500).send({ error: 'Error updating message.' })
	}
})

app.post('/create-run/:threadId', async (req, res) => {
	const { threadId } = req.params
	const { assistant_id, stream, tools } = req.body

	// 构造创建运行的参数
	const params = {
		assistant_id,
		...(stream ? { stream } : {}),
		...(tools ? { tools } : {}),
	}

	try {
		// 根据是否启用stream和是否提供tools参数来调用API
		const runPromise = openai.beta.threads.runs.create(threadId, params)
		let run

		if (stream) {
			const events = []
			for await (const event of await runPromise) {
				events.push(event)
			}
			run = { events }
		} else {
			run = (await runPromise).data
		}

		res.json(run)
	} catch (error) {
		console.error(error)
		res.status(500).send({ error: '创建运行时发生错误。' })
	}
})

app.post('/create-and-run-thread', async (req, res) => {
	const { assistant_id, stream, tools } = req.body
	const messages = req.body.thread ? req.body.thread.messages : []

	// 构造创建并运行线程的参数
	const params = {
		assistant_id,
		thread: {
			messages,
		},
		...(stream ? { stream } : {}),
		...(tools ? { tools } : {}),
	}

	try {
		// 根据是否启用stream和是否提供tools参数来调用API
		const createAndRunPromise = openai.beta.threads.createAndRun(params)
		let createAndRun

		if (stream) {
			const events = []
			for await (const event of await createAndRunPromise) {
				events.push(event)
			}
			createAndRun = { events }
		} else {
			createAndRun = (await createAndRunPromise).data
		}

		res.json(createAndRun)
	} catch (error) {
		console.error(error)
		res.status(500).send({ error: '创建并运行线程时发生错误。' })
	}
})

app.get('/list-runs/:threadId', async (req, res) => {
	const { threadId } = req.params

	try {
		const runs = await openai.beta.threads.runs.list(threadId)
		res.json(runs.data)
	} catch (error) {
		console.error(error)
		res.status(500).send({ error: '列出线程运行时发生错误。' })
	}
})

app.get('/list-run-steps/:threadId/:runId', async (req, res) => {
	const { threadId, runId } = req.params

	try {
		const runStep = await openai.beta.threads.runs.steps.list(threadId, runId)
		res.json(runStep.data)
	} catch (error) {
		console.error(error)
		res.status(500).send({ error: '列出运行步骤时发生错误。' })
	}
})

app.get('/retrieve-run/:threadId/:runId', async (req, res) => {
	const { threadId, runId } = req.params

	try {
		const run = await openai.beta.threads.runs.retrieve(threadId, runId)
		res.json(run.data)
	} catch (error) {
		console.error(error)
		res.status(500).send({ error: '检索运行信息时发生错误。' })
	}
})

app.get('/retrieve-run-step/:threadId/:runId/:stepId', async (req, res) => {
	const { threadId, runId, stepId } = req.params

	try {
		const runStep = await openai.beta.threads.runs.steps.retrieve(
			threadId,
			runId,
			stepId
		)
		res.json(runStep.data)
	} catch (error) {
		console.error(error)
		res.status(500).send({ error: '检索运行步骤信息时发生错误。' })
	}
})

app.put('/update-run/:threadId/:runId', async (req, res) => {
	const { threadId, runId } = req.params
	const { metadata } = req.body

	if (!metadata) {
		return res.status(400).send('元数据为必填项。')
	}

	try {
		const run = await openai.beta.threads.runs.update(threadId, runId, {
			metadata,
		})
		res.json(run.data)
	} catch (error) {
		console.error(error)
		res.status(500).send({ error: '更新运行信息时发生错误。' })
	}
})

app.post('/submit-tool-outputs/:threadId/:runId', async (req, res) => {
	const { threadId, runId } = req.params
	const { tool_outputs, stream } = req.body

	try {
		if (stream) {
			// 假设以流的形式提交工具输出并处理响应
			const responseStream = await openai.beta.threads.runs.submitToolOutputs(
				threadId,
				runId,
				{ tool_outputs }
			)

			const events = []
			for await (const event of responseStream) {
				events.push(event)
			}
			res.json(events)
		} else {
			// 以非流式提交工具输出
			const run = await openai.beta.threads.runs.submitToolOutputs(
				threadId,
				runId,
				{ tool_outputs }
			)
			res.json(run.data)
		}
	} catch (error) {
		console.error(error)
		res.status(500).send({ error: '提交工具输出时发生错误。' })
	}
})

app.post('/cancel-run/:threadId/:runId', async (req, res) => {
	const { threadId, runId } = req.params

	try {
		const run = await openai.beta.threads.runs.cancel(threadId, runId)
		res.json({ success: true, message: '运行已成功取消', run: run.data })
	} catch (error) {
		console.error(error)
		res.status(500).send({ error: '取消运行时发生错误。' })
	}
})

// ===== Grok API 相关路由 =====

// 获取可用的 Grok API 列表
app.get('/grok/clients', (req, res) => {
	try {
		const availableClients = getAvailableGrokClients()
		const clientsInfo = getGrokClientsInfo()
		const supportedModels = getSupportedModels()
		
		res.json({
			availableClients,
			clientsInfo,
			supportedModels,
			count: availableClients.length,
			modelCount: supportedModels.length
		})
	} catch (error) {
		console.error('获取 Grok 客户端列表时出错:', error)
		res.status(500).json({ error: '获取 Grok 客户端列表失败' })
	}
})

// Grok 聊天完成 - 使用指定的客户端
app.post('/grok/chat-completion', async (req, res) => {
	let isAborted = false

	// 创建 AbortController 实例
	const controller = new AbortController()
	const { signal } = controller

	// 监听请求中止事件
	req.on('aborted', () => {
		console.log('客户端已取消 Grok 请求')
		isAborted = true
		controller.abort()
	})

	try {
		const { model, messages, stream, stop, grokClient } = req.body
		
		// 检查是否有可用的 Grok 客户端
		if (!hasAvailableGrokClients()) {
			return res.status(503).json({ 
				error: '没有可用的 Grok API 客户端',
				message: '请检查 .env 文件中的 Grok API 配置'
			})
		}

		// 选择 Grok 客户端
		let selectedGrokClient
		let clientSelectionMethod = 'auto'
		
		if (grokClient) {
			// 使用指定的客户端
			selectedGrokClient = getGrokClient(grokClient)
			clientSelectionMethod = 'manual'
			if (!selectedGrokClient) {
				return res.status(400).json({ 
					error: `指定的 Grok 客户端 '${grokClient}' 不存在或未启用`,
					availableClients: getAvailableGrokClients(),
					supportedModels: getSupportedModels()
				})
			}
		} else {
			// 智能选择：优先根据模型选择最佳客户端
			selectedGrokClient = getBestGrokClientForModel(model)
			clientSelectionMethod = 'model-based'
			
			// 如果没有找到最佳客户端，使用负载均衡
			if (!selectedGrokClient) {
				selectedGrokClient = getNextGrokClient()
				clientSelectionMethod = 'load-balance'
			}
		}

		console.log(`使用 Grok 客户端进行聊天完成，模型: ${model || 'grok-4-latest'}`)

		// 构建请求参数
		const params = {
			model: model || 'grok-4-latest',
			messages,
			...(stream ? { stream } : {}),
			...(stop ? { stop } : {}),
		}

		// 调用 Grok API
		const grokResponse = await selectedGrokClient.chat.completions.create(params, {
			signal,
			responseType: stream ? 'stream' : undefined,
		})

		if (stream) {
			// 流式响应处理
			if (!grokResponse || typeof grokResponse[Symbol.asyncIterator] !== 'function') {
				console.error('Grok 流式响应无效')
				return res.status(500).json({ error: 'Grok 流式响应无效' })
			}

			// 设置 SSE 响应头
			res.setHeader('Content-Type', 'text/event-stream')
			res.setHeader('Cache-Control', 'no-cache')
			res.setHeader('Connection', 'keep-alive')
			res.flushHeaders()

			try {
				for await (const chunk of grokResponse) {
					if (isAborted) break

					const data = JSON.stringify(chunk)
					res.write(`data: ${data}\n\n`)
				}

				if (!isAborted) {
					console.log('Grok 数据流结束')
					res.write('data: [DONE]\n\n')
					res.end()
				} else {
					console.log('由于请求取消，提前结束 Grok 响应')
					res.end()
				}
			} catch (streamError) {
				console.error('处理 Grok 流式响应时出错:', streamError)
				if (!res.headersSent) {
					res.status(500).json({ error: '处理 Grok 流式响应时出错' })
				}
			}
		} else {
			// 非流式响应
			res.json(grokResponse)
		}
	} catch (error) {
		console.error('Grok API 调用出错:', error)
		
		if (error.name === 'AbortError') {
			console.log('Grok API 请求被中止')
			return res.status(499).json({ error: '请求被中止' })
		}

		// 处理不同类型的错误
		let errorMessage = 'Grok API 调用失败'
		let statusCode = 500

		if (error.status) {
			statusCode = error.status
			errorMessage = error.message || errorMessage
		}

		res.status(statusCode).json({ 
			error: errorMessage,
			details: error.message,
			type: 'grok_api_error'
		})
	}
})

// Grok 聊天完成 - 简化版本（自动选择客户端）
app.post('/grok/chat', async (req, res) => {
	try {
		const { model, messages, stream = false } = req.body
		
		if (!hasAvailableGrokClients()) {
			return res.status(503).json({ 
				error: '没有可用的 Grok API 客户端',
				message: '请检查 .env 文件中的 Grok API 配置'
			})
		}

		// 使用负载均衡选择客户端
		const grokClient = getNextGrokClient()
		
		console.log(`使用 Grok 进行简化聊天，模型: ${model || 'grok-4-latest'}`)

		const response = await grokClient.chat.completions.create({
			model: model || 'grok-4-latest',
			messages,
			stream
		})

		res.json(response)
	} catch (error) {
		console.error('Grok 简化聊天出错:', error)
		res.status(500).json({ 
			error: 'Grok 简化聊天失败',
			details: error.message
		})
	}
})

// 监听端口
const PORT = process.env.PORT || 8000
const HOST = process.env.HOST || '0.0.0.0'
server.listen(PORT, HOST, () => {
	console.log(`Your server is running on PORT ${PORT}`)
})
