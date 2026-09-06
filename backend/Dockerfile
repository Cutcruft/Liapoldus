FROM golang:1.26.6-bookworm AS build

WORKDIR /src

COPY go.mod go.sum ./
RUN go mod download

COPY . .
RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -trimpath -ldflags="-s -w" -o /out/liapoldus-server ./cmd/server

FROM alpine:3.20

RUN addgroup -S app && adduser -S -G app app

WORKDIR /app
COPY --from=build /out/liapoldus-server /app/liapoldus-server

USER app
EXPOSE 8080 18080
HEALTHCHECK --interval=10s --timeout=3s --start-period=5s --retries=3 \
  CMD wget -qO- http://localhost:8080/healthz >/dev/null 2>&1 || exit 1
ENTRYPOINT ["/app/liapoldus-server"]