# syntax=docker/dockerfile:1

# ---------- build stage ----------
# BUILDPLATFORM keeps the compile native even when cross-building for another
# architecture, which is much faster than emulating the toolchain under QEMU.
FROM --platform=$BUILDPLATFORM golang:1.25-alpine AS build

# Injected by docker/build-push-action for each --platform entry.
ARG TARGETOS
ARG TARGETARCH

# Injected by CI so the binary can report what it was built from.
ARG VERSION=dev
ARG COMMIT=none
ARG BUILD_DATE=unknown

WORKDIR /src

# Copy the module files first: this layer only busts when dependencies change.
COPY go.mod go.sum ./
RUN --mount=type=cache,target=/go/pkg/mod \
    go mod download && go mod verify

COPY . .

# CGO_ENABLED=0 produces a fully static binary, which is what the distroless
# "static" base image expects. -trimpath keeps builds reproducible.
RUN --mount=type=cache,target=/go/pkg/mod \
    --mount=type=cache,target=/root/.cache/go-build \
    CGO_ENABLED=0 GOOS=${TARGETOS} GOARCH=${TARGETARCH} \
    go build \
      -trimpath \
      -ldflags="-s -w \
        -X github.com/ValentinoTriadi/ci-cd-example/internal/version.Version=${VERSION} \
        -X github.com/ValentinoTriadi/ci-cd-example/internal/version.Commit=${COMMIT} \
        -X github.com/ValentinoTriadi/ci-cd-example/internal/version.BuildDate=${BUILD_DATE}" \
      -o /out/server ./cmd/server

# ---------- runtime stage ----------
# distroless/static: no shell, no package manager, runs as uid 65532.
FROM gcr.io/distroless/static-debian12:nonroot AS runtime

COPY --from=build /out/server /usr/local/bin/server

USER nonroot:nonroot
EXPOSE 8080
ENV ADDR=":8080" LOG_LEVEL="info"

ENTRYPOINT ["/usr/local/bin/server"]
