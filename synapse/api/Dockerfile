ARG REGISTRY=docker.io
FROM ${REGISTRY}/golang:1.24-alpine AS builder

WORKDIR /app

ARG GOPROXY=
ENV GOPROXY=${GOPROXY}
ENV GOPRIVATE=starclaw.net
ENV GONOSUMCHECK=starclaw.net
ENV GONOSUMDB=starclaw.net

COPY go.mod go.sum ./
COPY pheromone-sdk/ ./pheromone-sdk/
RUN go mod download

COPY . .
RUN CGO_ENABLED=0 GOOS=linux go build -ldflags="-X main.version=$(date +%Y.%m%d.%H%M)" -o /star-ai-api ./cmd/server

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
