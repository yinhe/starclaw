// config/bullQueues.js

import Bull from 'bull'

const redisConfig = {
	host: process.env.REDIS_HOST || '127.0.0.1',
	port: parseInt(process.env.REDIS_PORT, 10) || 6379,
	password: process.env.REDIS_PASSWORD || undefined,
	// 如果使用 ACL，添加用户名
	// username: process.env.REDIS_USERNAME || undefined,
}

// 初始化 Bull 队列
const imageToImageQueue = new Bull('image-to-image-queue', {
	redis: redisConfig,
})
const videoQueue = new Bull('video-generation', { redis: redisConfig })

export { imageToImageQueue, videoQueue }
