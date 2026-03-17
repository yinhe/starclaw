// config/multer.js

import multer from 'multer'
import path from 'path'
import fs from 'fs'
import { fileURLToPath } from 'url'

const __filename = fileURLToPath(import.meta.url)
const dirname = path.dirname(path.dirname(__filename)) // 跳转到项目根目录

const dataDir = process.env.PROXY_DATA_DIR || dirname

// 定义上传目录
const uploadDir = path.join(dataDir, 'uploads')
if (!fs.existsSync(uploadDir)) {
	fs.mkdirSync(uploadDir, { recursive: true })
}

// 配置 multer
const storage = multer.diskStorage({
	destination: (req, file, cb) => {
		cb(null, uploadDir)
	},
	filename: (req, file, cb) => {
		const uniqueSuffix = Date.now() + '-' + Math.round(Math.random() * 1e9)
		cb(null, uniqueSuffix + path.extname(file.originalname))
	},
})

const upload = multer({
	storage: storage,
	limits: { fileSize: 512 * 1024 * 1024 }, // 512MB
	fileFilter: (req, file, cb) => {
		const { purpose } = req.body
		// 根据不同的用途允许不同的文件类型
		if (purpose === 'fine-tune' || purpose === 'search') {
			if (path.extname(file.originalname).toLowerCase() !== '.jsonl') {
				return cb(
					new Error('用于 fine-tune 或 search 的文件必须是 .jsonl 格式。'),
					false
				)
			}
		} else if (purpose === 'assistants' || purpose === 'vision') {
			const supportedFormats = [
				'.doc',
				'.docx',
				'.pdf',
				'.jpg',
				'.jpeg',
				'.png',
			]
			if (
				!supportedFormats.includes(
					path.extname(file.originalname).toLowerCase()
				)
			) {
				return cb(
					new Error(
						'用于 assistants 或 vision 的文件必须是 .doc, .docx, .pdf, .jpg, .jpeg, 或 .png 格式。'
					),
					false
				)
			}
		} else if (purpose === 'parse') {
			// 允许解析的文件类型
			const supportedParseFormats = [
				'.pdf',
				'.doc',
				'.docx',
				'.xlsx',
				'.pptx',
				'.jpg',
				'.jpeg',
				'.png',
				'.mp3',
				'.wav',
				'.m4a',
				'.flac',
			]
			if (
				!supportedParseFormats.includes(
					path.extname(file.originalname).toLowerCase()
				)
			) {
				return cb(
					new Error(
						'用于 parse 的文件类型仅支持 PDF, DOC, DOCX, XLSX, PPTX, JPG, JPEG, PNG, MP3, WAV, M4A, FLAC。'
					),
					false
				)
			}
		}
		cb(null, true)
	},
})

export { upload, dirname }
