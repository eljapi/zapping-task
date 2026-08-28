FROM golang:1.25-alpine AS build

WORKDIR /src

COPY go.mod go.sum ./
RUN go mod download

COPY . .
RUN CGO_ENABLED=0 go build -trimpath -o /out/server ./cmd/server

FROM alpine:3.21

RUN adduser -D -u 10001 app

WORKDIR /app
COPY --from=build /out/server ./server
COPY web ./web

USER app

ENV LISTEN_ADDR=:8080 \
    WEB_DIR=/app/web \
    SEGMENTS_DIR=/segments

EXPOSE 8080

ENTRYPOINT ["/app/server"]
