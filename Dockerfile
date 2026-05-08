# syntax=docker/dockerfile:1.7

# ---- build stage ----
FROM golang:1.26-alpine AS build
WORKDIR /src
RUN apk add --no-cache build-base
# Cache deps
COPY go.mod go.sum ./
RUN --mount=type=cache,target=/go/pkg/mod \
    go mod download
# Source
COPY cmd ./cmd
COPY srv ./srv
COPY db  ./db
# Build static binary (modernc.org/sqlite is pure Go — CGO not required)
ENV CGO_ENABLED=0 GOOS=linux
RUN --mount=type=cache,target=/root/.cache/go-build \
    --mount=type=cache,target=/go/pkg/mod \
    go build -trimpath -ldflags="-s -w" -o /out/jobpilot ./cmd/srv

# ---- runtime stage ----
FROM gcr.io/distroless/static-debian12:nonroot
WORKDIR /app
COPY --from=build /out/jobpilot /app/jobpilot
# Persistent data lives in /data (mount a volume here)
ENV JOBPILOT_CONFIG=/data/jobpilot.config.json
WORKDIR /data
EXPOSE 8000
USER nonroot:nonroot
ENTRYPOINT ["/app/jobpilot", "-listen", ":8000"]
