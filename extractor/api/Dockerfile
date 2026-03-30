FROM golang:1.24-alpine AS builder

RUN apk add --no-cache git

WORKDIR /app
COPY go.mod go.sum ./
RUN go mod download

COPY . .
RUN CGO_ENABLED=0 go build -o extractor-api ./cmd/server

FROM alpine:3.20
RUN apk add --no-cache ca-certificates tzdata
COPY --from=builder /app/extractor-api /usr/local/bin/extractor-api

EXPOSE 8097
CMD ["extractor-api"]
