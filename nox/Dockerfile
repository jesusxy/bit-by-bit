# --- Build Stage ---
FROM golang:1.24-alpine AS builder

# set the working dir inside container
WORKDIR /app

# copy go.mod and go.sum files to download deps
COPY go.mod go.sum ./
RUN go mod download

# copy the rest of source code
COPY . .

# build go app creating static binary
RUN CGO_ENABLED=0 GOOS=linux go build -o /nox ./cmd/nox/main.go

# --- Final Stage ---

# use a minimal, non-root image for the final product
FROM gcr.io/distroless/static-debian11

# set working directory
WORKDIR /app

# copy the compiled binary from the biulder stage
COPY --from=builder /nox /nox

# copy geo ip db so the app can access
COPY testdata/GeoLite2-City.mmdb ./testdata/GeoLite2-City.mmdb

# cmd to run app once the container starts
CMD ["/nox"]