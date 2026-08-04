# --platform=$BUILDPLATFORM pins the builder to the runner's own architecture
# so Go cross-compiles instead of running under QEMU emulation. Emulated arm64
# Go builds take tens of minutes; cross-compiling takes the same time as a
# native build.
FROM --platform=$BUILDPLATFORM golang:1.22-alpine AS builder

# Supplied automatically by buildx for each requested platform.
ARG TARGETOS
ARG TARGETARCH

WORKDIR /usr/src/app/

COPY go.mod go.sum ./

# The cache mounts belong on the commands that actually populate them. They
# were previously attached to a bare mkdir, so every build re-downloaded every
# module and recompiled the whole dependency tree.
RUN --mount=type=cache,target=/go/pkg/mod \
    go mod download

COPY . .

# CGO_ENABLED=0 is required: cross-compiling with cgo would need an arm64
# toolchain in the builder image, and it also yields a static binary that does
# not depend on the release stage's libc.
RUN --mount=type=cache,target=/go/pkg/mod \
    --mount=type=cache,target=/root/.cache/go-build \
    CGO_ENABLED=0 GOOS=${TARGETOS} GOARCH=${TARGETARCH} \
    go build -o product-catalog .

####################################

FROM alpine AS release

WORKDIR /usr/src/app/

COPY ./products/ ./products/
# No trailing slash on the source: product-catalog is a file, not a directory.
COPY --from=builder /usr/src/app/product-catalog ./

# Default port. Must match containerPort/targetPort in kubernetes/productcatalog/*.yaml.
# Docker Compose overrides this with PRODUCT_CATALOG_PORT from .env (3550).
ENV PRODUCT_CATALOG_PORT=8080
EXPOSE 8080
ENTRYPOINT [ "./product-catalog" ]
