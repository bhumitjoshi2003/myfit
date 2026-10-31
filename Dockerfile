# syntax=docker/dockerfile:1
ARG GO_IMAGE=golang:1.26.10-bookworm@sha256:23ce71337b726608afb1bc00986e630a05034a87a87ea1fc2cd1ae5cf33387eb
ARG NODE_IMAGE=node:22-bookworm-slim@sha256:4b150b48b4116e2a22f2c39cc27025fbf4f153e2078fd3b14d159fdb5c7279da
FROM --platform=$BUILDPLATFORM ${NODE_IMAGE} AS frontend-build
WORKDIR /src/frontend
COPY frontend/package.json frontend/package-lock.json ./
RUN npm ci
COPY frontend/ ./
RUN npm run build

# Export this stage locally; the existing host Nginx serves these files.
FROM scratch AS frontend-assets
COPY --from=frontend-build /src/frontend/dist/ /

FROM --platform=$BUILDPLATFORM ${GO_IMAGE} AS api-build
ARG TARGETOS
ARG TARGETARCH
WORKDIR /src/backend
COPY backend/go.mod backend/go.sum ./
RUN go mod download
COPY backend/ ./
RUN CGO_ENABLED=0 GOOS=$TARGETOS GOARCH=$TARGETARCH go build -trimpath -ldflags="-s -w" -o /out/myfit .

FROM scratch AS runtime
ARG RELEASE_ID=development
ARG SOURCE_DIGEST=unrecorded
LABEL org.opencontainers.image.title="MyFit API" org.opencontainers.image.version=$RELEASE_ID org.opencontainers.image.revision=$SOURCE_DIGEST
COPY --from=api-build /etc/ssl/certs/ca-certificates.crt /etc/ssl/certs/ca-certificates.crt
COPY --from=api-build /out/myfit /app/myfit
COPY database/migrations/ /app/migrations/
USER 65532:65532
WORKDIR /app
ENV API_ADDR=0.0.0.0:8080 MIGRATIONS_DIR=/app/migrations
EXPOSE 8080
HEALTHCHECK --interval=30s --timeout=4s --start-period=45s --retries=3 CMD ["/app/myfit", "healthcheck"]
ENTRYPOINT ["/app/myfit"]
