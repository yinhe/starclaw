ARG REGISTRY=docker.io
FROM ${REGISTRY}/library/golang:1.24-alpine AS builder
WORKDIR /app
COPY go.mod ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 go build -o pheromone-api ./cmd/server

FROM ${REGISTRY}/library/alpine:3.21
RUN apk add --no-cache ca-certificates
WORKDIR /app
COPY --from=builder /app/pheromone-api .
EXPOSE 8100
CMD ["./pheromone-api"]
