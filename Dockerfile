# syntax=docker/dockerfile:1.7
ARG WEB_STAGE=web-build
ARG BINARY_STAGE=app-build

FROM --platform=$BUILDPLATFORM oven/bun:1.3.9 AS web-build
WORKDIR /src/web
COPY web/package.json web/bun.lock ./
RUN --mount=type=cache,target=/root/.bun/install/cache bun install --frozen-lockfile
COPY VERSION CHANGELOG.md README.md /src/
COPY assets /src/assets
COPY web ./
ARG VITE_TLDRAW_LICENSE_KEY
ARG BUILD_VERSION
ARG BUILD_COMMIT=unknown
ARG BUILD_TIME=unknown
ENV VITE_TLDRAW_LICENSE_KEY=$VITE_TLDRAW_LICENSE_KEY CANVAS_BUILD_VERSION=$BUILD_VERSION CANVAS_BUILD_COMMIT=$BUILD_COMMIT CANVAS_BUILD_TIME=$BUILD_TIME
RUN bun --bun ./node_modules/vite/bin/vite.js build

# CI builds the frontend once and supplies the same files to both architectures.
FROM scratch AS web-prebuilt
COPY backend/internal/webui/dist /src/web/dist
FROM ${WEB_STAGE} AS web-assets

FROM golang:1.25-alpine3.22 AS payment-package-build
ARG GOPROXY=https://proxy.golang.org,direct
ENV GOPROXY=$GOPROXY
RUN apk add --no-cache nodejs unzip zip
WORKDIR /src
COPY backend ./backend
COPY plugin-packages ./plugin-packages
ARG TARGETOS
ARG TARGETARCH
RUN --mount=type=cache,target=/root/.cache/go-build \
    PAYMENT_PLUGIN_GOOS="$TARGETOS" PAYMENT_PLUGIN_GOARCH="$TARGETARCH" PAYMENT_PLUGIN_CGO_ENABLED=0 \
    sh plugin-packages/build-packages.sh --payments-only

FROM alpine:3.22 AS payment-package-smoke
ARG TARGETOS
ARG TARGETARCH
RUN apk add --no-cache unzip
COPY --from=payment-package-build /src/plugin-packages /src/plugin-packages
RUN PAYMENT_PLUGIN_SMOKE_TEST=1 sh /src/plugin-packages/verify-payment-packages.sh "$TARGETOS" "$TARGETARCH"

FROM node:22.19-alpine3.22 AS bundled-runtime
RUN apk add --no-cache tar
WORKDIR /bundle/agent-runtime/pi
COPY backend/agent-runtime/pi/package.json backend/agent-runtime/pi/package-lock.json ./
RUN npm ci --omit=dev --ignore-scripts
COPY backend/agent-runtime/pi/agent-runtime.mjs ./
COPY --from=payment-package-smoke /src/plugin-packages/*.yingce-plugin /bundle/plugin-packages/
# The embedded extractor accepts only directories and regular files.
RUN tar --dereference --hard-dereference -czf /runtime.tar.gz -C /bundle agent-runtime/pi plugin-packages

FROM golang:1.25-alpine3.22 AS app-build
ARG GOPROXY=https://proxy.golang.org,direct
ENV GOPROXY=$GOPROXY
RUN apk add --no-cache build-base
WORKDIR /src/backend
COPY backend/go.mod backend/go.sum ./
COPY backend/third_party ./third_party
RUN go mod download
COPY backend ./
COPY VERSION /src/VERSION
COPY --from=web-assets /src/web/dist ./internal/webui/dist
COPY --from=bundled-runtime /runtime.tar.gz ./internal/bundled/runtime.tar.gz
ARG BUILD_VERSION
ARG BUILD_COMMIT=unknown
ARG BUILD_TIME=unknown
RUN --mount=type=cache,target=/root/.cache/go-build \
    version="${BUILD_VERSION:-$(tr -d '\r\n' </src/VERSION)}"; \
    CGO_ENABLED=1 go build -tags embedweb -trimpath \
    -ldflags="-s -w -X yingce/backend/internal/buildinfo.Version=$version -X yingce/backend/internal/buildinfo.Commit=$BUILD_COMMIT -X yingce/backend/internal/buildinfo.BuildTime=$BUILD_TIME" \
    -o /out/yingce-backend ./cmd/server

FROM scratch AS binary-export
COPY --from=app-build /out/yingce-backend /yingce-backend

# Release images consume the exported binary, rather than building it a second time.
FROM scratch AS binary-prebuilt
COPY dist/yingce-backend /out/yingce-backend
FROM ${BINARY_STAGE} AS binary-assets

FROM node:22.19-alpine3.22 AS runtime
RUN apk add --no-cache ca-certificates tzdata wget ffmpeg su-exec \
    && addgroup -S app && adduser -S app -G app
WORKDIR /app
ENV GIN_MODE=release CANVAS_BACKEND_ADDR=:8080 CANVAS_BACKEND_DATA_DIR=/data CANVAS_MANAGED_BINARY_PATH=/data/system/bin/yingce-backend
COPY --from=binary-assets /out/yingce-backend /opt/canvas/yingce-backend
COPY docker/canvas-entrypoint.sh /usr/local/bin/canvas-entrypoint
RUN chmod 0755 /usr/local/bin/canvas-entrypoint /opt/canvas/yingce-backend
ENTRYPOINT ["/usr/local/bin/canvas-entrypoint"]
EXPOSE 8080
HEALTHCHECK --interval=30s --timeout=3s --start-period=30s --retries=3 CMD wget -qO- http://127.0.0.1:8080/api/health/ready >/dev/null || exit 1
