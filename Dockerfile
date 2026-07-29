# Build stage
FROM golang:1.26-alpine AS builder

WORKDIR /app

COPY go.mod go.sum ./
RUN go mod download

COPY . .

# Two binaries: the API and the migration runner. Migrations are a separate step so
# they complete before the API accepts traffic.
#
# -s -w drops the symbol table and DWARF data, which are not useful in the image and
# account for roughly a quarter of the binary.
RUN CGO_ENABLED=0 GOOS=linux go build -ldflags="-s -w" -o /out/server ./cmd/api \
    && CGO_ENABLED=0 GOOS=linux go build -ldflags="-s -w" -o /out/migrate ./cmd/migrate

# Final runtime stage
FROM alpine:3.20

RUN adduser -D -u 10001 app

WORKDIR /app

COPY --from=builder /out/server /out/migrate ./

USER app

EXPOSE 8080

CMD ["./server"]
