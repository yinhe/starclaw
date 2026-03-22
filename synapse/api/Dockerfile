ARG REGISTRY=docker.io
FROM ${REGISTRY}/golang:1.24-alpine AS builder

WORKDIR /app

ARG GOPROXY=
ENV GOPROXY=${GOPROXY}

COPY go.mod go.sum ./
RUN go mod download

COPY . .
RUN CGO_ENABLED=0 GOOS=linux go build -o /star-ai-api ./cmd/server

ARG REGISTRY=docker.io
FROM ${REGISTRY}/alpine:latest

RUN sed -i 's|dl-cdn.alpinelinux.org|mirrors.aliyun.com|g' /etc/apk/repositories
RUN apk --no-cache add ca-certificates tzdata

WORKDIR /app

COPY --from=builder /star-ai-api .
COPY configs/config.yaml ./configs/
COPY providers/ ./providers/
COPY certs/ ./certs/

EXPOSE 8096

CMD ["./star-ai-api"]
