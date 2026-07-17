# syntax=docker/dockerfile:1
FROM golang:1.22-alpine AS builder

WORKDIR /src

ARG VERSION=dev
ARG COMMIT_SHA=unknown
ARG BUILD_DATE=unknown
ARG TARGETOS
ARG TARGETARCH

RUN apk add --no-cache git

COPY go.mod go.sum ./
RUN go mod download

COPY . .

RUN CGO_ENABLED=0 GOOS=${TARGETOS:-linux} GOARCH=${TARGETARCH:-amd64} \
    go build -ldflags "-s -w -X main.version=${VERSION} -X main.commitSHA=${COMMIT_SHA} -X main.buildDate=${BUILD_DATE}" \
    -o /out/clusterprobe-chaos-ctrl ./cmd/chaos-ctrl

FROM gcr.io/distroless/static-debian12:nonroot

ARG VERSION=dev
ARG COMMIT_SHA=unknown
ARG BUILD_DATE=unknown

LABEL org.opencontainers.image.title="clusterprobe-chaos-ctrl" \
      org.opencontainers.image.version="${VERSION}" \
      org.opencontainers.image.revision="${COMMIT_SHA}" \
      org.opencontainers.image.created="${BUILD_DATE}" \
      org.opencontainers.image.source="https://github.com/Ghvinerias/clusterprobe"

WORKDIR /
COPY --from=builder /out/clusterprobe-chaos-ctrl /clusterprobe-chaos-ctrl

USER 65534:65534

ENTRYPOINT ["/clusterprobe-chaos-ctrl"]
