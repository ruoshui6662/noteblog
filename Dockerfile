# syntax=docker/dockerfile:1
FROM --platform=$BUILDPLATFORM node:22-bookworm-slim AS frontend
WORKDIR /src/web
COPY web/package.json web/package-lock.json ./
RUN npm ci
COPY web/ ./
RUN npm run build

FROM --platform=$BUILDPLATFORM golang:1.26.5-bookworm AS backend
WORKDIR /src
COPY go.mod ./
COPY cmd/ ./cmd/
COPY web/*.go ./web/
COPY --from=frontend /src/web/dist ./web/dist
RUN test -z "$(gofmt -l cmd web/*.go)" && go vet -tags production ./... && go test -tags production ./...
ARG TARGETOS
ARG TARGETARCH
RUN CGO_ENABLED=0 GOOS=$TARGETOS GOARCH=$TARGETARCH go build -buildvcs=false -tags production -trimpath -ldflags="-s -w" -o /out/markdown-docs ./cmd/server

FROM alpine:3.23 AS runtime
RUN apk add --no-cache ca-certificates \
    && addgroup -g 10001 app \
    && adduser -D -H -u 10001 -G app app \
    && mkdir -p /data/content /data/media /data/backups \
    && chown -R 10001:10001 /data
COPY --from=backend /out/markdown-docs /usr/local/bin/markdown-docs
ENV APP_ADDR=0.0.0.0:8080 APP_DATA_DIR=/data APP_ENV=container
USER 10001:10001
WORKDIR /data
EXPOSE 8080
HEALTHCHECK --interval=30s --timeout=5s --start-period=10s --retries=3 CMD ["/usr/local/bin/markdown-docs", "healthcheck"]
ENTRYPOINT ["/usr/local/bin/markdown-docs"]
