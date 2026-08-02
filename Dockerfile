# ---- build stage ----
FROM golang:1.23-alpine AS build
WORKDIR /src

# Cache module downloads. Requires go.sum (run `make tidy` before building).
COPY go.mod go.sum ./
RUN go mod download

COPY . .
RUN CGO_ENABLED=0 go build -o /out/api ./cmd/api

# ---- run stage ----
# distroless/static: tiny base image, no shell, ideal for a CGO-free binary.
FROM gcr.io/distroless/static-debian12
COPY --from=build /out/api /api
EXPOSE 8080
ENTRYPOINT ["/api"]
