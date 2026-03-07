FROM golang:1.24-alpine AS builder

WORKDIR /app

# Use China Go proxy if GOPROXY is set (pass --build-arg GOPROXY=https://goproxy.cn,direct)
ARG GOPROXY=
ENV GOPROXY=${GOPROXY}

COPY go.mod go.sum ./
RUN go mod download

COPY . .
RUN CGO_ENABLED=0 GOOS=linux go build -o /starclaw ./cmd/server

FROM alpine:latest

# Use Alibaba Cloud mirrors for faster downloads in China
RUN sed -i 's|dl-cdn.alpinelinux.org|mirrors.aliyun.com|g' /etc/apk/repositories

# Core + Browser + FFmpeg + Python + Node.js (one layer)
RUN apk --no-cache add \
    ca-certificates tzdata bash curl wget git unzip \
    chromium nss freetype harfbuzz font-noto-cjk ffmpeg libass \
    python3 py3-pip py3-setuptools \
    nodejs npm \
    && python3 -m pip install --break-system-packages -i https://mirrors.aliyun.com/pypi/simple/ --trusted-host mirrors.aliyun.com flask fastapi uvicorn requests beautifulsoup4 aiohttp dashscope \
    && npm config set registry https://registry.npmmirror.com && npm install -g yarn pnpm typescript ts-node

# Go (copy from builder)
COPY --from=builder /usr/local/go /usr/local/go
ENV PATH="/usr/local/go/bin:${PATH}"
ENV GOPATH="/tmp/gopath"
ENV CHROME_BIN=/usr/bin/chromium-browser

WORKDIR /app

# These layers change on every code update (fast, no package installs)
COPY --from=builder /starclaw .
COPY configs/config.yaml ./configs/
COPY plugins/ ./plugins/
COPY scripts/ ./scripts/

EXPOSE 8080

CMD ["./starclaw"]
