// src/socket.js
import { Server } from 'socket.io'

let io = null

export const initSocket = (server) => {
	io = new Server(server, {
		cors: {
			origin: '*', // 根据需要配置
			methods: ['GET', 'POST'],
		},
	})
	return io
}

export const getIO = () => {
	if (!io) {
		throw new Error('Socket.io not initialized')
	}
	return io
}
