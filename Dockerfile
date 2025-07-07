FROM golang:1.24-alpine AS builder

RUN apk add --no-cache git

WORKDIR /app

COPY go.mod go.sum ./
RUN go mod download

COPY . .

RUN CGO_ENABLED=0 GOOS=linux go build -a -installsuffix cgo -ldflags="-s -w" -o pterodactyl-backup-cleaner .

FROM alpine:latest

RUN apk add --no-cache ca-certificates tzdata nfs-utils

WORKDIR /app

COPY --from=builder /app/pterodactyl-backup-cleaner .

RUN mkdir -p /mnt/pterodactyl

VOLUME ["/mnt/pterodactyl"]

CMD ["./pterodactyl-backup-cleaner"]
