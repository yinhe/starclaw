// config/falClient.js

import { fal } from '@fal-ai/client'
import dotenv from 'dotenv'

dotenv.config()

const FAL_KEY = process.env.FAL_KEY

if (!FAL_KEY) {
	console.error('Fal AI Key 未设置！请在 .env 文件中设置 FAL_KEY。')
	process.exit(1)
}

fal.config({
	credentials: FAL_KEY,
})

export default fal
