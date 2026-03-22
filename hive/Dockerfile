FROM golang:1.24-alpine AS builder

RUN apk add --no-cache git

WORKDIR /build
COPY go.mod go.sum ./
RUN go mod download

COPY . .
RUN CGO_ENABLED=0 go build -ldflags="-s -w" -o hive-controller ./cmd/main.go

FROM alpine:3.19
RUN apk add --no-cache ca-certificates tzdata docker-cli
WORKDIR /app
COPY --from=builder /build/hive-controller .

EXPOSE 9090
CMD ["./hive-controller"]
