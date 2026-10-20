# Stage 1: build
FROM golang:1.27 AS builder
WORKDIR /app
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 GOOS=linux go build -trimpath -ldflags="-s -w" -o /out/server ./cmd/server

# Stage 2: run
FROM alpine:3.22
RUN addgroup -S app && adduser -S app -G app
USER app
COPY --from=builder /out/server /server
EXPOSE 8080
CMD ["/server"]