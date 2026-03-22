// config/middlewares.js

import rateLimit from 'express-rate-limit'
import dotenv from 'dotenv'

dotenv.config()

// 白名单IP列表
const whitelist = [
	'127.0.0.1',
	'47.103.51.32',
	'149.28.214.192',
	'45.32.129.250',
	'10.12.96.3',
]

// 设置速率限制
const limiter = rateLimit({
	windowMs: 15 * 60 * 1000, // 15分钟时间窗口
	max: 100, // 每个IP最多只能在15分钟内发起100个请求
	message: {
		error: 'Too many requests, please try again later.',
	},
	// 检查请求是否来自白名单IP
	skip: (req) => whitelist.includes(req.ip), // 白名单IP将跳过速率限制
})

export { limiter }
