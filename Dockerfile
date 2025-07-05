FROM golang:1.23.4-alpine AS builder
RUN apk add --no-cache git make gcc musl-dev
WORKDIR /app
COPY go.mod go.sum ./
RUN go mod download && go mod verify
COPY . .
RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 \
    go build -ldflags="-s -w" -trimpath -o /app/video-bot

FROM alpine:3.18
WORKDIR /app
RUN apk add --no-cache ffmpeg python3 ca-certificates \
    && update-ca-certificates \
    && wget https://github.com/yt-dlp/yt-dlp/releases/latest/download/yt-dlp -O /usr/local/bin/yt-dlp \
    && chmod a+rx /usr/local/bin/yt-dlp \
    && yt-dlp --update
COPY --from=builder /app/video-bot /app/
COPY config.yaml /app/
COPY cookies.txt /app/
RUN mkdir -p /app/downloads && chown -R nobody:nobody /app
USER nobody
ENTRYPOINT ["/app/video-bot"]