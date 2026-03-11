FROM golang:1.24-alpine AS builder
WORKDIR /build
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 go build -o /queen-api ./cmd/server/

FROM alpine:3.19
RUN apk add --no-cache ca-certificates tzdata
ENV TZ=Asia/Shanghai
WORKDIR /app
COPY --from=builder /queen-api .
COPY configs/ ./configs/
COPY certs/ ./certs/
EXPOSE 8085
CMD ["./queen-api"]
