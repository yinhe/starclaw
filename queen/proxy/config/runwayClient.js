// config/runwayClient.js

import RunwayML from '@runwayml/sdk'
import dotenv from 'dotenv'

dotenv.config()

const RUNWAYML_API_SECRET = process.env.RUNWAYML_API_SECRET

if (!RUNWAYML_API_SECRET) {
	console.error(
		'RunwayML API Secret 未设置！请在 .env 文件中设置 RUNWAYML_API_SECRET。'
	)
	process.exit(1)
}

const runwayClient = new RunwayML({
	apiSecret: RUNWAYML_API_SECRET,
})

export default runwayClient
