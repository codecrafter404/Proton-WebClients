# Stage 1: Build the frontend
FROM node:22-alpine AS frontend-builder

RUN corepack enable && corepack prepare yarn@4.9.1 --activate

WORKDIR /src
# Copy only the files needed for yarn install first (for caching)
COPY package.json yarn.lock .yarnrc.yml ./
COPY .yarn .yarn
COPY packages packages
COPY applications/calendar applications/calendar
COPY tsconfig.base.json tsconfig.webpack.json turbo.json ./

RUN yarn install --immutable || yarn install

# Build the calendar frontend in standalone mode
RUN yarn workspace proton-calendar build --appMode=standalone --api=/api

# Stage 2: Build the Go binary
FROM golang:1.24-alpine AS backend-builder

RUN apk add --no-cache gcc musl-dev

WORKDIR /app
COPY backend/go.mod backend/go.sum ./
RUN go mod download

COPY backend/ .
RUN CGO_ENABLED=1 go build -o calendar-api .

# Stage 3: Minimal runtime image
FROM alpine:3.21

RUN apk add --no-cache ca-certificates tzdata

RUN adduser -D -u 1000 appuser
WORKDIR /app

COPY --from=backend-builder /app/calendar-api .
COPY --from=backend-builder /app/static ./static
COPY --from=frontend-builder /src/applications/calendar/dist ./web

RUN mkdir -p /data && chown appuser:appuser /data

USER appuser

EXPOSE 8080

ENTRYPOINT ["./calendar-api"]
CMD ["-addr", ":8080", "-db", "/data/calendar.db", "-web", "./web"]
