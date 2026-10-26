# syntax=docker/dockerfile:1

# 1. Frontend: build the React Router SPA into static files.
FROM node:24-alpine AS web
WORKDIR /src/web
COPY web/package.json web/package-lock.json ./
RUN npm ci
COPY web/ ./
RUN npm run build

# 2. Backend: build a static Go binary.
FROM golang:1.27.3-alpine AS backend
WORKDIR /src/backend
COPY backend/go.mod backend/go.sum* ./
RUN go mod download
COPY backend/ ./
RUN CGO_ENABLED=0 go build -trimpath -ldflags="-s -w" -o /out/shelfmark ./cmd/shelfmark

# 3. Runtime: the Go binary serves the API and the built SPA.
FROM gcr.io/distroless/static-debian12:nonroot
WORKDIR /app
COPY --from=backend /out/shelfmark /app/shelfmark
COPY --from=web /src/web/build/client /app/public
ENV PORT=8080 \
    STATIC_DIR=/app/public
EXPOSE 8080
USER nonroot:nonroot
ENTRYPOINT ["/app/shelfmark"]
