# syntax=docker/dockerfile:1

FROM golang:1.25.3 AS build
ENV GOTOOLCHAIN=auto
WORKDIR /src
COPY go.mod go.sum ./
RUN --mount=type=cache,target=/go/pkg/mod go mod download
COPY . .
RUN --mount=type=cache,target=/go/pkg/mod --mount=type=cache,target=/root/.cache/go-build \
    CGO_ENABLED=0 go build -trimpath -ldflags "-s -w" -o /out/pellets ./cmd/app
RUN mkdir -p /out/data/backups

FROM gcr.io/distroless/static:nonroot
WORKDIR /app
COPY --from=build /out/pellets /app/pellets
COPY --from=build /out/data /data
ENV PELLETS_DATA_FILE=/data/pellets.json \
    PELLETS_BACKUP_DIR=/data/backups \
    PELLETS_LISTEN_ADDR=0.0.0.0:8080
EXPOSE 8080
ENTRYPOINT ["/app/pellets"]
