# ---- frontend build stage ----
# Build the React/TS SPA so it can be embedded into the Go binary.
FROM node:20-alpine AS web
WORKDIR /web
COPY web/package.json web/package-lock.json ./
RUN npm ci
COPY web/ ./
RUN npm run build                      # -> /web/dist

# ---- backend build stage ----
FROM golang:1.25-alpine AS build
WORKDIR /src

# Cache module downloads. Requires go.sum (run `make tidy` before building).
COPY go.mod go.sum ./
RUN go mod download

COPY . .
# Bake the freshly built SPA in via //go:embed web/dist (see web/embed.go).
# Overwrites any local/stale dist; .dockerignore keeps the local one out.
COPY --from=web /web/dist ./web/dist
RUN CGO_ENABLED=0 go build -o /out/api ./cmd/api

# ---- run stage ----
# distroless/static: tiny base image, no shell, ideal for a CGO-free binary.
FROM gcr.io/distroless/static-debian12
COPY --from=build /out/api /api
EXPOSE 8080
ENTRYPOINT ["/api"]
