// config/grokClient.js

import OpenAI from 'openai'
import dotenv from 'dotenv'

dotenv.config()

// 仅保留 Grok-4 配置
const GROK_CONFIGS = [
	{
		name: 'grok-4',
		apiKey: process.env.GROK_API_KEY_4,
		baseURL: process.env.GROK_BASE_URL_4 || 'https://api.x.ai/v1',
		enabled: !!process.env.GROK_API_KEY_4,
		models: ['grok-4-0709', 'grok-4', 'grok-4-latest'],
	},
]

// 创建 Grok 客户端实例
const grokClients = {}
const enabledGrokConfigs = GROK_CONFIGS.filter(config => config.enabled)

enabledGrokConfigs.forEach(config => {
	try {
		grokClients[config.name] = new OpenAI({
			apiKey: config.apiKey,
			baseURL: config.baseURL,
		})
		console.log(`✅ Grok API ${config.name} 已初始化`)
	} catch (error) {
		console.error(`❌ 初始化 Grok API ${config.name} 失败:`, error.message)
	}
})

// 获取可用的 Grok 客户端列表
function getAvailableGrokClients() {
	return Object.keys(grokClients)
}

// 根据名称获取 Grok 客户端
function getGrokClient(name) {
	return grokClients[name]
}

// 获取默认的 Grok 客户端（第一个可用的）
function getDefaultGrokClient() {
	const availableClients = getAvailableGrokClients()
	return availableClients.length > 0 ? grokClients[availableClients[0]] : null
}

// 负载均衡：轮询选择 Grok 客户端
let currentClientIndex = 0
function getNextGrokClient() {
	const availableClients = getAvailableGrokClients()
	if (availableClients.length === 0) return null
	
	const clientName = availableClients[currentClientIndex % availableClients.length]
	currentClientIndex++
	return grokClients[clientName]
}

// 根据模型选择最佳的 Grok 客户端
function getBestGrokClientForModel(modelName) {
	if (!modelName) return getDefaultGrokClient()
	
	// 查找支持该模型的客户端
	for (const config of GROK_CONFIGS) {
		if (config.enabled && config.models.includes(modelName)) {
			return grokClients[config.name]
		}
	}
	
	// 如果没有找到支持该模型的客户端，返回默认客户端
	return getDefaultGrokClient()
}

// 获取所有支持的模型列表
function getSupportedModels() {
	const models = new Set()
	GROK_CONFIGS.forEach(config => {
		if (config.enabled) {
			config.models.forEach(model => models.add(model))
		}
	})
	return Array.from(models)
}

// 获取客户端信息（包括支持的模型）
function getGrokClientsInfo() {
	return GROK_CONFIGS.filter(config => config.enabled).map(config => ({
		name: config.name,
		baseURL: config.baseURL,
		enabled: config.enabled,
		models: config.models
	}))
}

// 检查是否有可用的 Grok API
function hasAvailableGrokClients() {
	return getAvailableGrokClients().length > 0
}

export {
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
