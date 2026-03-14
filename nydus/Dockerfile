FROM golang:1.24-alpine AS builder
RUN apk add --no-cache git
WORKDIR /build
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 go build -o /nydus-server ./cmd/nydus-server/

FROM alpine:3.19
RUN apk add --no-cache git curl openssh-server openssh-client shadow bash && \
    adduser -D -s /usr/bin/git-shell git && \
    mkdir -p /data/nydus/repos /home/git/.ssh && \
    chown -R git:git /data/nydus /home/git/.ssh && \
    chmod 700 /home/git/.ssh && \
    ssh-keygen -A
WORKDIR /app
COPY --from=builder /nydus-server .
COPY nydus.yaml .
COPY scripts/entrypoint.sh /entrypoint.sh
RUN chmod +x /entrypoint.sh
EXPOSE 8095 22
ENTRYPOINT ["/entrypoint.sh"]
