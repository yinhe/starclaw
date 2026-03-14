// routes/imageToImageRoutes.js

import express from 'express'
import Joi from 'joi'
import { fal, redisClient, imageToImageQueue } from '../config/index.js'
import { getIO } from '../socket.js' // 导入共享的 io 实例
import path from 'path'
import fs from 'fs'

const router = express.Router()

// 定义请求体验证模式
const submitSchema = Joi.object({
	model: Joi.string()
		.pattern(/^fal-ai\/.+/)
		.required()
		.messages({
			'string.pattern.base': 'model 参数格式不正确。',
			'any.required': 'model 是必填项。',
		}),
	image_url: Joi.string().uri().required().messages({
		'string.uri': 'image_url 必须是有效的 URI。',
		'any.required': 'image_url 是必填项。',
	}),
	prompt: Joi.string().max(512).required().messages({
		'string.max': 'prompt 的长度不能超过 512 个字符。',
		'any.required': 'prompt 是必填项。',
	}),
	strength: Joi.number().min(0).max(1).default(0.95),
	num_inference_steps: Joi.number().integer().min(1).default(40),
	seed: Joi.number().integer(),
	guidance_scale: Joi.number().min(0).default(3.5),
	sync_mode: Joi.boolean().default(false),
	num_images: Joi.number().integer().min(1).max(10).default(1),
	enable_safety_checker: Joi.boolean().default(true),
	webhookUrl: Joi.string().uri().optional(),
})

// 提交图生图请求
router.post('/submit', async (req, res) => {
	// 验证请求体
	const { error, value } = submitSchema.validate(req.body)
	if (error) {
		return res.status(400).json({ error: error.details[0].message })
	}

	const {
		model,
		image_url,
		prompt,
		strength,
		num_inference_steps,
		seed,
		guidance_scale,
		sync_mode,
		num_images,
		enable_safety_checker,
		webhookUrl,
	} = value

	try {
		if (sync_mode) {
			// 同步模式：等待结果
			const result = await fal.subscribe(model, {
				input: {
					image_url,
					prompt,
					strength,
					num_inference_steps,
					seed,
					guidance_scale,
					num_images,
					enable_safety_checker,
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
				status: 'SUCCEEDED',
				images: JSON.stringify(result.data.images),
				prompt: result.data.prompt || '',
				seed: result.data.seed != null ? result.data.seed.toString() : '',
				timings: JSON.stringify(result.data.timings),
				has_nsfw_concepts: JSON.stringify(result.data.has_nsfw_concepts),
			})

			// 通过 Socket.IO 通知前端
			const io = getIO()
			io.to(result.requestId).emit('taskCompleted', {
				images: result.data.images,
				prompt: result.data.prompt,
				seed: result.data.seed,
				timings: result.data.timings,
				has_nsfw_concepts: result.data.has_nsfw_concepts,
			})

			// 返回结果
			return res.status(200).json({
				images: result.data.images,
				prompt: result.data.prompt,
				seed: result.data.seed,
				timings: result.data.timings,
				has_nsfw_concepts: result.data.has_nsfw_concepts,
			})
		} else {
			// 异步模式：提交任务到队列
			const { request_id } = await fal.queue.submit(model, {
				input: {
					image_url,
					prompt,
					strength,
					num_inference_steps,
					seed,
					guidance_scale,
					num_images,
					enable_safety_checker,
				},
				webhookUrl: webhookUrl || null,
			})

			console.log('Submit Result:', { request_id })

			// 保存任务状态到 Redis
			await redisClient.hSet(`task:${request_id}`, {
				status: 'RUNNING',
				images: JSON.stringify([]),
				prompt: prompt || '',
				seed: seed != null ? seed.toString() : '',
				timings: JSON.stringify({}),
				has_nsfw_concepts: JSON.stringify([]),
			})

			// 添加任务到 Bull 队列
			await imageToImageQueue.add(
				{ taskId: request_id, modelPath: model },
				{
					attempts: 3, // 重试次数
					backoff: 5000, // 重试间隔（毫秒）
				}
			)

			// 返回 request_id
			return res.status(202).json({ request_id })
		}
	} catch (err) {
		console.error('Error submitting image-to-image request:', err)
		return res.status(500).json({ error: '提交图像生成请求失败。' })
	}
})

// 获取请求状态
router.post('/status', async (req, res) => {
	const { model, requestId, logs } = req.body

	// 验证必填参数
	if (!model || !requestId) {
		return res.status(400).json({ error: 'model 和 requestId 是必填项。' })
	}

	// 验证模型格式
	const modelPattern = /^fal-ai\/.+/
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

// 获取请求结果
router.post('/result', async (req, res) => {
	const { model, requestId } = req.body

	// 验证必填参数
	if (!model || !requestId) {
		return res.status(400).json({ error: 'model 和 requestId 是必填项。' })
	}

	// 验证模型格式
	const modelPattern = /^fal-ai\/.+/
	if (!modelPattern.test(model)) {
		return res.status(400).json({ error: 'model 参数格式不正确。' })
	}

	try {
		const result = await fal.queue.result(model, {
			requestId,
		})

		console.log('Result:', result.data)
		console.log('Request ID:', result.requestId)

		// 更新 Redis 中任务状态为 'SUCCEEDED'
		if (result.data) {
			await redisClient.hSet(`task:${requestId}`, {
				status: 'SUCCEEDED',
				images: JSON.stringify(result.data.images),
				prompt: result.data.prompt || '',
				seed: result.data.seed != null ? result.data.seed.toString() : '',
				timings: JSON.stringify(result.data.timings),
				has_nsfw_concepts: JSON.stringify(result.data.has_nsfw_concepts),
			})

			// 通过 Socket.IO 通知前端
			const io = getIO()
			io.to(requestId).emit('taskCompleted', {
				images: result.data.images,
				prompt: result.data.prompt,
				seed: result.data.seed,
				timings: result.data.timings,
				has_nsfw_concepts: result.data.has_nsfw_concepts,
			})
		}

		res.status(200).json({
			images: result.data.images,
			prompt: result.data.prompt,
			seed: result.data.seed,
			timings: result.data.timings,
			has_nsfw_concepts: result.data.has_nsfw_concepts,
		})
	} catch (error) {
		console.error('获取请求结果失败:', error)

		// 更新 Redis 中任务状态为 'FAILED'
		await redisClient.hSet(`task:${requestId}`, {
			status: 'FAILED',
			error: error.message || '获取请求结果失败。',
		})

		// 通过 Socket.IO 通知前端
		const io = getIO()
		io.to(requestId).emit('taskFailed', {
			error: error.message || '获取请求结果失败。',
		})

		res.status(500).json({ error: '获取请求结果失败。' })
	}
})

// 取消任务的路由
router.delete('/cancel', async (req, res) => {
	const { request_id } = req.body

	if (!request_id) {
		return res.status(400).json({ error: 'request_id 是必填项。' })
	}

	try {
		// 从 Bull 队列中获取任务
		const job = await imageToImageQueue.getJob(request_id)

		if (job) {
			const { modelPath } = job.data // 获取 modelPath

			// 移除队列中的 Job（适用于等待中的任务）
			await job.remove()

			// 调用 Fal AI 的取消 API
			await fal.queue.cancel(modelPath, { requestId: request_id })

			// 更新 Redis 中任务状态为 'canceled'
			await redisClient.hSet(`task:${request_id}`, {
				status: 'canceled',
			})

			// 通过 Socket.IO 通知前端
			const io = getIO()
			io.to(request_id).emit('taskCanceled', {
				message: '任务已取消。',
			})

			return res.status(200).json({ message: '任务已取消。' })
		} else {
			// 如果 Job 不在等待队列中，检查是否在活跃队列中
			const activeJobs = await imageToImageQueue.getActive()
			const jobToCancel = activeJobs.find((j) => j.id === request_id)

			if (jobToCancel) {
				const { modelPath } = jobToCancel.data // 获取 modelPath

				// 标记 Job 为失败，并传递取消原因
				await jobToCancel.moveToFailed({ message: '任务已取消。' }, true)

				// 调用 Fal AI 的取消 API
				await fal.queue.cancel(modelPath, { requestId: request_id })

				// 更新 Redis 中任务状态为 'canceled'
				await redisClient.hSet(`task:${request_id}`, {
					status: 'canceled',
				})

				// 通过 Socket.IO 通知前端
				const io = getIO()
				io.to(request_id).emit('taskCanceled', {
					message: '正在处理中的任务已取消。',
				})

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

// 定义轮询函数
const pollImageToImageTaskStatus = async (
	modelPath,
	requestId,
	interval = 5000,
	maxAttempts = 20
) => {
	let attempts = 0

	while (attempts < maxAttempts) {
		try {
			// 获取任务状态
			const statusResponse = await fal.queue.status(modelPath, {
				requestId,
				logs: false,
			})

			console.log(`任务状态（${requestId}）：`, statusResponse)

			if (
				statusResponse.status === 'SUCCEEDED' ||
				statusResponse.status === 'COMPLETED'
			) {
				// 任务成功，获取结果
				const result = await fal.queue.result(modelPath, { requestId })
				return { success: true, data: result.data }
			} else if (statusResponse.status === 'FAILED') {
				// 任务失败
				return { success: false, error: statusResponse.error || '任务失败。' }
			} else {
				// 任务仍在进行中，等待
				attempts += 1
				await new Promise((resolve) => setTimeout(resolve, interval))
			}
		} catch (error) {
			console.error(`轮询任务状态时出错（${requestId}）：`, error)
			return { success: false, error: error.message || '轮询任务状态时出错。' }
		}
	}

	// 超时
	return { success: false, error: '任务超时未完成。' }
}

// 修改 Bull 队列的工作进程
imageToImageQueue.process(async (job) => {
	const { taskId, modelPath } = job.data

	try {
		// 轮询任务状态，直到完成
		const pollResult = await pollImageToImageTaskStatus(modelPath, taskId)

		if (pollResult.success) {
			const resultData = pollResult.data

			console.log(`任务完成，ID: ${taskId}`, resultData)

			// 更新任务状态并保存到 Redis
			await redisClient.hSet(`task:${taskId}`, {
				status: 'SUCCEEDED',
				images: JSON.stringify(resultData.images),
				prompt: resultData.prompt || '',
				seed: resultData.seed != null ? resultData.seed.toString() : '',
				timings: JSON.stringify(resultData.timings),
				has_nsfw_concepts: JSON.stringify(resultData.has_nsfw_concepts),
			})

			// 通过 Socket.IO 通知前端
			const io = getIO()
			io.to(taskId).emit('taskCompleted', {
				images: resultData.images,
				prompt: resultData.prompt,
				seed: resultData.seed,
				timings: resultData.timings,
				has_nsfw_concepts: resultData.has_nsfw_concepts,
			})
		} else {
			// 任务失败
			await redisClient.hSet(`task:${taskId}`, {
				status: 'FAILED',
				error: pollResult.error,
			})

			console.error(`任务失败，ID: ${taskId}，错误: ${pollResult.error}`)

			// 通过 Socket.IO 通知前端
			const io = getIO()
			io.to(taskId).emit('taskFailed', {
				error: pollResult.error,
			})
		}
	} catch (error) {
		// 更新任务状态为 FAILED
		await redisClient.hSet(`task:${taskId}`, {
			status: 'FAILED',
			error: error.message || '图像生成失败。',
		})

		console.error(`任务失败，ID: ${taskId}`, error)

		// 通过 Socket.IO 通知前端
		const io = getIO()
		io.to(taskId).emit('taskFailed', {
			error: error.message || '图像生成失败。',
		})
	}
})

export default router
