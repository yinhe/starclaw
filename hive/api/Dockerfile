FROM golang:1.25-alpine AS builder

RUN apk add --no-cache git

# Copy carapace dependency first (referenced via replace directive)
WORKDIR /build/carapace
COPY carapace/go.mod carapace/go.sum ./
COPY carapace/*.go ./
COPY carapace/backend/ ./backend/

# Copy pheromone/sdk dependency (referenced via replace directive)
WORKDIR /build/pheromone/sdk
COPY pheromone/sdk/go.mod pheromone/sdk/go.sum ./
COPY pheromone/sdk/*.go ./

# Copy hive/api source
WORKDIR /build/hive/api
COPY hive/api/go.mod hive/api/go.sum ./
RUN go mod download

COPY hive/api/ .
RUN CGO_ENABLED=0 go build -ldflags="-s -w" -o /build/hive-controller ./cmd/main.go

FROM alpine:3.19
RUN apk add --no-cache ca-certificates tzdata docker-cli util-linux
WORKDIR /app
COPY --from=builder /build/hive-controller .

EXPOSE 9090
CMD ["./hive-controller"]
