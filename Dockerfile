FROM golang:1.25-alpine AS builder

WORKDIR /app

COPY go.mod go.sum ./
RUN go mod download

COPY . .

RUN go build -o server ./cmd/main.go

FROM alpine:latest

WORKDIR /root/

# Install ca-certificates (needed for Firestore/Auth HTTPS calls)
RUN apk --no-cache add ca-certificates

COPY --from=builder /app/server .

# Copy environment file (optional, if you want to bake it in,
# otherwise docker-compose injects env vars)
# COPY .env .

EXPOSE 3000

# Run the server
CMD ["./server"]