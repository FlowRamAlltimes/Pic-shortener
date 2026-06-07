FROM golang:1.26.3-alpine AS builder
RUN apk add --no-cache git ca-certificates
WORKDIR /build
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 GOOS=linux go build -o pic-service app.go

FROM alpine:latest
RUN apk add --no-cache ca-certificates tzdata
WORKDIR /app
COPY --from=builder /build/pic-service .
COPY --from=builder /build/config.yml .
RUN mkdir -p storage/originals storage/cache logs
EXPOSE 10000
ENTRYPOINT [ "./pic-service" ]
