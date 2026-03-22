// config/redis.js

import { createClient } from 'redis'
import dotenv from 'dotenv'

dotenv.config()

// Support both REDIS_URL (docker-compose) and REDIS_HOST/REDIS_PORT
const redisUrl = process.env.REDIS_URL
const redisOpts = redisUrl
	? { url: redisUrl }
	: {
		socket: {
			host: process.env.REDIS_HOST || '127.0.0.1',
			port: parseInt(process.env.REDIS_PORT, 10) || 6379,
		},
		password: process.env.REDIS_PASSWORD || undefined,
	}

const redisClient = createClient(redisOpts)

redisClient.on('error', (err) => console.error('Redis Client Error', err))

// 连接 Redis
const connectRedis = async () => {
	if (!redisClient.isOpen) {
		try {
			await redisClient.connect()
			console.log('Redis 客户端已连接')
		} catch (err) {
			console.error('Redis 连接失败:', err)
			process.exit(1) // 退出应用程序，避免在未连接 Redis 的情况下运行
		}
	}
}

connectRedis()

export default redisClient
