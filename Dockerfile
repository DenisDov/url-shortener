# syntax=docker/dockerfile:1

ARG GO_VERSION=1.26.5
ARG ALPINE_VERSION=3.24

FROM golang:${GO_VERSION}-alpine${ALPINE_VERSION} AS build

# Declared inside the stage on purpose: an ARG before the first FROM is global
# and would not be visible here. .git is in .dockerignore, so the build cannot
# derive this itself -- CI passes it with --build-arg.
ARG VERSION=dev

WORKDIR /src

COPY go.mod go.sum ./
RUN --mount=type=cache,target=/go/pkg/mod \
    go mod download

COPY . .
RUN --mount=type=cache,target=/go/pkg/mod \
    --mount=type=cache,target=/root/.cache/go-build \
    CGO_ENABLED=0 go build -trimpath \
      -ldflags="-s -w -X main.version=${VERSION}" -o /out/api ./cmd/api

FROM gcr.io/distroless/static-debian12:nonroot

COPY --from=build /out/api /api

EXPOSE 8080

USER nonroot:nonroot
ENTRYPOINT ["/api"]
