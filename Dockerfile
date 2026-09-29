# syntax=docker/dockerfile:1
# Imagem multi-arch do forja (amd64 + arm64) com binário ESTÁTICO com cgo (musl).
#   docker buildx build --platform linux/amd64,linux/arm64 -t REG/forja:TAG --push .
#
# O estágio de build roda SEMPRE na arquitetura da máquina ($BUILDPLATFORM) e
# faz compilação cruzada com o "xx": sem emular o compilador inteiro no QEMU.
FROM --platform=$BUILDPLATFORM docker.io/tonistiigi/xx:1.6.1 AS xx

FROM --platform=$BUILDPLATFORM docker.io/library/golang:1.26.8-alpine AS build
COPY --from=xx / /
RUN apk add --no-cache clang lld
ARG TARGETPLATFORM
ARG VERSION=dev
# gcc/musl-dev da arquitetura de DESTINO (xx-apk instala a versão certa)
RUN xx-apk add --no-cache gcc musl-dev linux-headers
WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY . .
ENV CGO_ENABLED=1
RUN xx-go build -trimpath \
    -ldflags "-s -w -X github.com/chwiee/forja/internal/cli.version=${VERSION} -linkmode external -extldflags '-static'" \
    -tags "containers_image_openpgp exclude_graphdriver_btrfs" \
    -o /out/forja ./cmd/forja && \
    xx-verify --static /out/forja

# policy.json, registries.conf e certificados CA já vêm embutidos no binário.
FROM gcr.io/distroless/static-debian13
COPY --from=build /out/forja /usr/local/bin/forja
WORKDIR /workspace
ENTRYPOINT ["/usr/local/bin/forja"]
