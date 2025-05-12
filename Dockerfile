FROM golang:1.22-alpine AS builder

WORKDIR /app
COPY go.mod go.sum ./
RUN go mod download
COPY . .

RUN go build -o /prometheus-telegram-bot ./cmd/main.go

FROM alpine:latest
WORKDIR /
COPY --from=builder /prometheus-telegram-bot /prometheus-telegram-bot

ENTRYPOINT ["/prometheus-telegram-bot"]
