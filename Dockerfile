# Multi-arch build. The UI is static (architecture-independent) so it builds
# once on the native build host; the Go binary is pure-Go (CGO disabled) so it
# cross-compiles for the target arch without emulation. Only the tiny final
# runtime layer is pulled per-arch. This keeps arm64 / arm/v7 (Raspberry Pi)
# builds fast and reliable.

# Stage 1: build the web UI on the build host.
FROM --platform=$BUILDPLATFORM node:22-alpine AS ui
WORKDIR /web
COPY web/package.json web/package-lock.json* ./
RUN npm install --no-fund --no-audit
COPY web/ ./
RUN npm run build

# Stage 2: cross-compile the Go binary for the target arch.
FROM --platform=$BUILDPLATFORM golang:1.25-alpine AS build
ARG TARGETOS TARGETARCH TARGETVARIANT
WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY . .
# Drop the built UI into the embed path (internal/server/web/dist).
COPY --from=ui /web/dist ./internal/server/web/dist
RUN CGO_ENABLED=0 GOOS=${TARGETOS} GOARCH=${TARGETARCH} GOARM=${TARGETVARIANT#v} \
    go build -trimpath -ldflags="-s -w" -o /out/hush ./cmd/hush

# Stage 3: minimal runtime for the target arch.
FROM alpine:3.21
RUN addgroup -S hush && adduser -S hush -G hush \
    && mkdir -p /data && chown hush:hush /data
COPY --from=build /out/hush /usr/local/bin/hush
VOLUME /data
ENV HUSH_DATA=/data
EXPOSE 4874
USER hush
ENTRYPOINT ["hush"]
CMD ["serve"]
