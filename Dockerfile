# Web UI build stage lands with the web milestone; the binary is the whole app.
FROM golang:1.25-alpine AS build
WORKDIR /src
COPY go.mod ./
COPY . .
RUN CGO_ENABLED=0 go build -trimpath -ldflags="-s -w" -o /out/hush ./cmd/hush

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
