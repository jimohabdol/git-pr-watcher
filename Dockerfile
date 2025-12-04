
FROM golang:1.24-alpine AS builder

RUN apk add --no-cache git ca-certificates

WORKDIR /app

COPY go.mod go.sum ./

RUN go mod download

COPY . .

RUN CGO_ENABLED=0 GOOS=linux go build -a -installsuffix cgo -o pr-watcher .

# Final stage
FROM alpine:latest

RUN apk --no-cache add ca-certificates

RUN adduser -D -s /bin/sh appuser

WORKDIR /app

COPY --from=builder /app/pr-watcher .

COPY --from=builder /app/config.yaml.example .

RUN chown -R appuser:appuser /app

USER appuser

EXPOSE 8080

CMD ["./pr-watcher"]
