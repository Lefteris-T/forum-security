FROM golang:1.25-alpine AS builder

WORKDIR /src

RUN apk add --no-cache gcc musl-dev

COPY go.mod go.sum ./
RUN go mod download

COPY . .

RUN CGO_ENABLED=1 GOOS=linux go build -o /forum ./cmd/forum


FROM alpine:3.22

RUN apk add --no-cache ca-certificates \
    && addgroup -S forum \
    && adduser -S forum -G forum

WORKDIR /app

COPY --from=builder /forum /app/forum
COPY migrations /app/migrations
COPY templates /app/templates
COPY static /app/static

RUN mkdir -p /app/data /app/static/uploads \
    && chown -R forum:forum /app

USER forum

EXPOSE 8443

ENV FORUM_ADDRESS=:8443
ENV FORUM_DATABASE_PATH=/app/data/forum.db
ENV FORUM_HTTPS_ENABLED=true
ENV FORUM_TLS_CERT_FILE=/run/certs/localhost.crt
ENV FORUM_TLS_KEY_FILE=/run/certs/localhost.key
ENV FORUM_SECURE_COOKIE=true

CMD ["/app/forum"]
