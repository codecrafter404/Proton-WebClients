# Stage 1: Build the Go binary
FROM golang:1.24-alpine AS builder

RUN apk add --no-cache gcc musl-dev

WORKDIR /app
COPY go.mod go.sum ./
RUN go mod download

COPY . .
RUN CGO_ENABLED=1 go build -o calendar-api .

# Stage 2: Minimal runtime image
FROM alpine:3.21

RUN apk add --no-cache ca-certificates tzdata

RUN adduser -D -u 1000 appuser
WORKDIR /app

COPY --from=builder /app/calendar-api .
COPY --from=builder /app/static ./static

RUN mkdir -p /data && chown appuser:appuser /data

USER appuser

EXPOSE 8080

ENTRYPOINT ["./calendar-api"]
CMD ["-addr", ":8080", "-db", "/data/calendar.db"]
