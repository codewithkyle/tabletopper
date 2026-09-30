FROM golang:1.25-alpine AS builder
WORKDIR /app
RUN apk add --no-cache build-base libwebp-dev
COPY ./server/go.mod ./server/go.sum ./
RUN go mod download
COPY ./server/. ./
ENV CGO_ENABLED=1
RUN go build -o tabletopper .
FROM alpine:3.24
RUN apk add --no-cache libwebp && adduser -D -H app
WORKDIR /srv/tabletopper
COPY --from=builder /app/tabletopper .
COPY ./server/public ./public
USER app
EXPOSE 3000
CMD ["./tabletopper"]
