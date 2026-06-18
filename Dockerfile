FROM golang:1.21-alpine AS builder
WORKDIR /app
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 go build -ldflags="-s -w" -o shark-dashboard .

FROM alpine:3.19
RUN apk --no-cache add ca-certificates tzdata
WORKDIR /app
COPY --from=builder /app/shark-dashboard .
COPY --from=builder /app/web ./web

EXPOSE 8080
ENTRYPOINT ["./shark-dashboard"]
