# syntax=docker/dockerfile:1

# Stage 1: Compile pure-Go static binary (CGO_ENABLED=0)
FROM golang:alpine AS builder
WORKDIR /src
ENV GOTOOLCHAIN=auto

# Download dependencies first for better layer caching
COPY go.mod go.sum ./
RUN go mod download

COPY . .
RUN CGO_ENABLED=0 GOOS=linux go build -trimpath -ldflags="-s -w" -o /out/server ./cmd/server

# Stage 2: Minimal distroless non-root runtime (~15 MB)
FROM gcr.io/distroless/static-debian12:nonroot
WORKDIR /app

COPY --from=builder /out/server /app/server
COPY --from=builder /src/assets /app/assets

ENV APP_ENV=prod
ENV TRUST_PROXY=true
ENV ASSETS_DIR=/app/assets
ENV PORT=8080

EXPOSE 8080
ENTRYPOINT ["/app/server"]
