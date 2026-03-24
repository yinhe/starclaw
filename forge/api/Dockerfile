FROM golang:1.24-alpine AS builder
WORKDIR /app
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=1 go build -o forge-api ./cmd/server/

FROM alpine:3.21
RUN apk add --no-cache ca-certificates sqlite-libs
WORKDIR /app
COPY --from=builder /app/forge-api .
EXPOSE 8099
CMD ["./forge-api"]
