FROM golang:1.24-alpine AS builder

WORKDIR /build
COPY go.mod go.sum ./
COPY pheromone-sdk/ ./pheromone-sdk/
RUN go mod download

COPY . .
RUN CGO_ENABLED=0 GOOS=linux go build -ldflags="-X main.version=$(date +%Y.%m%d.%H%M)" -o overlord-api ./cmd/server

FROM alpine:latest
RUN apk --no-cache add ca-certificates tzdata
ENV TZ=Asia/Shanghai
WORKDIR /app
COPY --from=builder /build/overlord-api .

EXPOSE 8095
CMD ["./overlord-api"]
