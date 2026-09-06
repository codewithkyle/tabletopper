# Step 1: Build
FROM golang:1.25-alpine AS builder

WORKDIR /app

# Native deps for CGO + libwebp
RUN apk add --no-cache build-base libwebp-dev

COPY ./server/go.mod ./server/go.sum ./
RUN go mod download

# Copy the rest of the source. TWO COPY LINES ARE ENOUGH BECAUSE NOTHING IS
# GENERATED IN HERE: templ's *_templ.go, sqlc's queries package, Tailwind's
# public/css/app.css and esbuild's public/static/journal-editor.js are all built
# outside the image and copied in with everything else, so no stage needs Node,
# esbuild, Tailwind, templ or sqlc.
#
# What it does need is a working tree where all four have been run -- which is
# what `make run` guarantees by depending on templ, sqlc, css and js before it
# reaches `docker compose up`. Building by hand means running those first.
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
