# Stage 1: build the web UI.
FROM node:22-alpine AS ui
WORKDIR /web
COPY web/package.json web/package-lock.json* ./
RUN npm install --no-fund --no-audit
COPY web/ ./
RUN npm run build

# Stage 2: build the Go binary with the UI embedded.
FROM golang:1.25-alpine AS build
WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY . .
# Drop the built UI into the embed path (internal/server/web/dist).
COPY --from=ui /web/dist ./internal/server/web/dist
RUN CGO_ENABLED=0 go build -trimpath -ldflags="-s -w" -o /out/hush ./cmd/hush

# Stage 3: minimal runtime.
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
