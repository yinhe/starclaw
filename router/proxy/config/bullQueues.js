// config/bullQueues.js

import Bull from 'bull'

// Support both REDIS_URL (docker-compose) and REDIS_HOST/REDIS_PORT
const redisUrl = process.env.REDIS_URL
const redisConfig = redisUrl
	? redisUrl
	: {
		host: process.env.REDIS_HOST || '127.0.0.1',
		port: parseInt(process.env.REDIS_PORT, 10) || 6379,
		password: process.env.REDIS_PASSWORD || undefined,
	}

// 初始化 Bull 队列
const imageToImageQueue = new Bull('image-to-image-queue', {
	redis: redisConfig,
})
const videoQueue = new Bull('video-generation', { redis: redisConfig })

export { imageToImageQueue, videoQueue }
