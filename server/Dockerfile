# Step 1: Build
FROM golang:1.25-alpine AS builder

WORKDIR /app

# Native deps for CGO + libwebp
RUN apk add --no-cache build-base libwebp-dev

COPY ./server/go.mod ./server/go.sum ./
RUN go mod download

# copy the rest of the source
COPY ./server/. ./

# Build with CGO enabled
ENV CGO_ENABLED=1
RUN go build -o tabletopper .

# Step 2: Run
FROM alpine:latest

# libwebp runtime libs are usually needed when dynamically linked
RUN apk add --no-cache libwebp

WORKDIR /root/
COPY --from=builder /app/tabletopper .
COPY ./server/public ./public

EXPOSE 3000
CMD ["./tabletopper"]
