# Build context is the repo root (docker build -f server/Dockerfile .) so
# this stage can reach both server/ and web/.

FROM node:22-alpine AS web-build
WORKDIR /web
COPY web/package.json web/package-lock.json ./
RUN npm ci
COPY web/ ./
RUN npm run build

FROM golang:1.26-alpine AS server-build
WORKDIR /server
COPY server/go.mod server/go.sum ./
RUN go mod download
COPY server/ ./
# CGO_ENABLED=0 + the pure-Go SQLite driver (modernc.org/sqlite) keeps this
# a fully static binary — no libc dependency, so it runs unmodified on the
# distroless base below (see ADR 0003).
RUN CGO_ENABLED=0 go build -o /out/retainer ./cmd/retainer

FROM gcr.io/distroless/static-debian12
COPY --from=server-build /out/retainer /retainer
COPY --from=web-build /web/dist /web/dist
VOLUME /data
ENV DB_PATH=/data/retainer.db
ENV STATIC_DIR=/web/dist
ENV ADDR=:8070
EXPOSE 8070
ENTRYPOINT ["/retainer"]
