ARG REGISTRY=docker.io
FROM ${REGISTRY}/golang:1.24-alpine AS builder

WORKDIR /app
RUN apk add --no-cache git

COPY go.mod go.sum* ./
COPY pheromone-sdk/ ./pheromone-sdk/
RUN go mod download 2>/dev/null || true

COPY . .
RUN CGO_ENABLED=0 GOOS=linux go build -o /drone-api ./cmd/server

FROM ${REGISTRY}/alpine:latest
RUN apk --no-cache add ca-certificates tzdata python3 py3-pip py3-requests py3-yaml
ENV TZ=Asia/Shanghai
WORKDIR /app

COPY --from=builder /drone-api .

# Copy Python collectors + cocoon for worker subprocess
COPY ../collectors/ /app/collectors/
COPY ../cocoon/ /app/cocoon/
COPY ../scripts/ /app/scripts/
COPY ../configs/ /app/configs/

EXPOSE 8110
CMD ["./drone-api"]
