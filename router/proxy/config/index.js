// config/index.js

import redisClient from './redis.js'
import { imageToImageQueue, videoQueue } from './bullQueues.js'
import { openai, API_KEY } from './openaiClient.js'
import fal from './falClient.js'
import runwayClient from './runwayClient.js'
import { limiter } from './middlewares.js'
import { upload, dirname } from './multer.js'
import {
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
} from './grokClient.js'

export {
	redisClient,
	imageToImageQueue,
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
}
