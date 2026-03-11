ARG REGISTRY=docker.io
FROM ${REGISTRY}/node:20-alpine

WORKDIR /app

# Install ffmpeg for video processing
RUN apk add --no-cache ffmpeg

COPY package.json ./
RUN npm install --production

COPY . .

# Data directories
RUN mkdir -p /app/data/uploads /app/data/videos /app/data/audio

EXPOSE 8000

CMD ["node", "server.js"]
