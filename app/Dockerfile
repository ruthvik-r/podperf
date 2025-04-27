FROM golang:1.22.12-alpine AS builder
WORKDIR /app
COPY . .
RUN go mod download
RUN CGO_ENABLED=0 GOOS=linux go build -o goapp .

# a minimal alpine image for the final image
FROM alpine:latest
WORKDIR /app
COPY --from=builder /app/goapp .
EXPOSE 8080
CMD ["./goapp"] 