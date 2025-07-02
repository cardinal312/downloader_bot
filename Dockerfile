# Этап сборки
FROM golang:1.23.4-alpine AS builder

# Устанавливаем необходимые инструменты для сборки
RUN apk add --no-cache \
    git \
    make \
    gcc \
    musl-dev

WORKDIR /app

# Копируем файлы зависимостей
COPY go.mod go.sum ./

# Скачиваем зависимости
RUN go mod download && go mod verify

# Копируем исходный код
COPY . .

# Собираем приложение с правильными флагами
RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 \
    go build \
    -ldflags="-s -w" \
    -trimpath \
    -o /app/video-bot

# Этап запуска
FROM alpine:3.18

WORKDIR /app

# Устанавливаем runtime-зависимости
RUN apk add --no-cache \
    ffmpeg \
    python3 \
    ca-certificates \
    && update-ca-certificates

# Устанавливаем последнюю версию yt-dlp
RUN wget https://github.com/yt-dlp/yt-dlp/releases/latest/download/yt-dlp -O /usr/local/bin/yt-dlp \
    && chmod a+rx /usr/local/bin/yt-dlp \
    && yt-dlp --update

# Копируем бинарник из этапа сборки
COPY --from=builder /app/video-bot /app/

# Копируем конфигурации
COPY config.yaml /app/
COPY --chown=nobody:nobody cookies.txt /app/  

# Создаем директорию для загрузок
RUN mkdir -p /app/downloads \
    && chown -R nobody:nobody /app

# Переключаемся на непривилегированного пользователя
USER nobody

# Точка входа
ENTRYPOINT ["/app/video-bot"]


# FROM golang:1.23.4-alpine AS builder
# WORKDIR /app

# # Установка зависимостей для сборки
# RUN apk add --no-cache git

# # Копируем файлы модулей
# COPY go.mod go.sum ./
# RUN go mod download

# # Копируем исходный код
# COPY . .

# # Собираем приложение
# RUN CGO_ENABLED=0 GOOS=linux go build -ldflags="-w -s" -o /app/video-bot

# # Этап запуска
# FROM alpine:3.18
# WORKDIR /app

# # Установка runtime-зависимостей
# RUN apk add --no-cache \
#     ffmpeg \
#     yt-dlp \
#     python3 \
#     ca-certificates \
#     tzdata \
#     && mkdir -p /app/downloads

# # Копируем бинарник и конфиг
# COPY --from=builder /app/video-bot /app/
# COPY config.yaml /app/

# # Настройка прав
# RUN chmod +x /app/video-bot

# # Точка входа
# ENTRYPOINT ["/app/video-bot"]