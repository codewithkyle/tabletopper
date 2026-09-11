# Step 1: Build
FROM golang:1.25-alpine AS builder
WORKDIR /app
# Native deps for CGO + libwebp
RUN apk add --no-cache build-base libwebp-dev
COPY ./server/go.mod ./server/go.sum ./
RUN go mod download
# Copy the rest of the source. TWO COPY LINES ARE ENOUGH BECAUSE NOTHING IS
# GENERATED IN HERE: templ's *_templ.go, sqlc's queries package, the generated
# js/room/protocol.ts, Tailwind's public/css/app.css and esbuild's two bundles
# under public/static are all built outside the image and copied in with
# everything else, so no stage needs Node, esbuild, Tailwind, templ or sqlc.
#
# What it does need is a working tree where all five have been run -- which is
# what `make run` guarantees by depending on templ, sqlc, protocol, css and js
# before it reaches `docker compose up`. Building by hand means running those
# first.
COPY ./server/. ./
# Build with CGO enabled
ENV CGO_ENABLED=1
RUN go build -o tabletopper .
# Step 2: Run
#
# PINNED, AND PINNED TO THE ALPINE THE BUILDER IS ON. `latest` was a base that
# moves: a rebuild months from now would pull whatever release is current and
# turn any incompatibility with the musl the binary was linked against into a
# load-time failure nobody changed anything to cause. golang:1.25-alpine is
# Alpine 3.24 today, so these two lines move together and they move
# deliberately.
FROM alpine:3.24
# libwebp is belt and braces rather than a requirement. The encoder comes from
# chai2010/webp, which compiles the C in through cgo -- `ldd ./tabletopper`
# inside this image reports musl and nothing else -- so the package is not what
# the binary loads. It stays because it costs a couple of megabytes and its
# absence would be a puzzle if a future dependency did link against the system
# copy.
#
# app is a system user with no password and no home: the process reads its
# binary and ./public and writes nothing to disk, so it needs no directory of
# its own, and root inside the container is one bug away from root on whatever
# is ever mounted into it.
RUN apk add --no-cache libwebp && adduser -D -H app
# NOT /root, WHICH IS root'S HOME AND UNREADABLE BY THE USER BELOW, AND NOT
# /app, WHICH compose.yaml BIND-MOUNTS THE REPOSITORY OVER. That mount is
# harmless where it lands now and was harmless before, when this was /root --
# nothing here reads the repository at runtime, because public/ is copied into
# the image. But a WORKDIR of /app is shadowed by it the moment the container
# starts, and the symptom is the binary not existing: `stat ./tabletopper: no
# such file or directory`, from an image that demonstrably contains it.
WORKDIR /srv/tabletopper
COPY --from=builder /app/tabletopper .
COPY ./server/public ./public
USER app
EXPOSE 3000
CMD ["./tabletopper"]
