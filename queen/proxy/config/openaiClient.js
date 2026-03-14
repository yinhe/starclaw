// config/openaiClient.js

import OpenAI from 'openai'
import dotenv from 'dotenv'

dotenv.config()

const OPENAI_API_KEY = process.env.OPENAI_API_KEY || process.env.API_KEY

if (!OPENAI_API_KEY) {
	console.error('OpenAI API Key 未设置！请在 .env 文件中设置 API_KEY。')
	process.exit(1)
}

const openai = new OpenAI({
	apiKey: OPENAI_API_KEY,
})

export { openai, OPENAI_API_KEY, OPENAI_API_KEY as API_KEY }
