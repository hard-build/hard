FROM golang:1.23.12-bookworm AS go

FROM ubuntu:22.04 AS llvm18-base

ARG DEBIAN_FRONTEND=noninteractive

RUN apt-get update \
    && apt-get install -y --no-install-recommends \
        ca-certificates \
        curl \
        gnupg \
    && curl --fail --location --retry 3 \
        --output /tmp/llvm-snapshot.gpg.key \
        https://apt.llvm.org/llvm-snapshot.gpg.key \
    && gpg --dearmor \
        --output /usr/share/keyrings/apt.llvm.org.gpg \
        /tmp/llvm-snapshot.gpg.key \
    && printf '%s\n' \
        'deb [signed-by=/usr/share/keyrings/apt.llvm.org.gpg] https://apt.llvm.org/jammy/ llvm-toolchain-jammy-18 main' \
        > /etc/apt/sources.list.d/apt.llvm.org.list \
    && rm -f /tmp/llvm-snapshot.gpg.key \
    && rm -rf /var/lib/apt/lists/*

FROM llvm18-base AS hard-builder

ARG HARD_VERSION=v4.0
ARG HARD_REVISION

COPY --from=go /usr/local/go /usr/local/go

ENV PATH=/usr/local/go/bin:$PATH

RUN apt-get update \
    && apt-get install -y --no-install-recommends \
        build-essential \
        libclang-18-dev \
    && rm -rf /var/lib/apt/lists/*

RUN test -n "${HARD_REVISION}" \
    && curl --fail --location --retry 3 \
        --output /tmp/hard.tar.gz \
        "https://github.com/hard-build/hard/archive/${HARD_REVISION}.tar.gz" \
    && install -d /tmp/hard-source \
    && tar -xzf /tmp/hard.tar.gz \
        --directory /tmp/hard-source \
        --strip-components=1 \
    && cd /tmp/hard-source/hard \
    && CGO_ENABLED=1 go build \
        -trimpath \
        -ldflags="-s -w" \
        -o /tmp/hard . \
    && /tmp/hard --help >/dev/null \
    && printf '%s\n' "${HARD_VERSION}" > /tmp/hard-version

FROM llvm18-base

ARG DEBIAN_FRONTEND=noninteractive
ARG HARD_VERSION=v4.0
ARG HARD_REVISION

LABEL org.opencontainers.image.source="https://github.com/hard-build/hard" \
      org.opencontainers.image.description="hard linux64 C++ build environment with glibc 2.35" \
      org.opencontainers.image.version="v4.0-glibc.2.35" \
      org.opencontainers.image.revision="${HARD_REVISION}"

RUN apt-get update \
    && apt-get install -y --no-install-recommends \
        autoconf \
        automake \
        build-essential \
        clang-format-18 \
        cmake \
        git \
        libclang-common-18-dev \
        libclang1-18 \
        libgmock-dev \
        libgtest-dev \
        libtool \
        libtool-bin \
        meson \
        ninja-build \
        pkg-config \
    && rm -rf /var/lib/apt/lists/*

RUN install -d \
        /hard \
        /project \
        /usr/local/libexec/hard/bin \
        /usr/local/libexec/hard/format \
        /usr/local/libexec/hard/licenses \
    && chmod 0777 /hard /project

COPY --from=hard-builder /tmp/hard /usr/local/libexec/hard/hard
COPY --from=hard-builder /tmp/hard-source/hard.h /usr/local/libexec/hard/hard.h
COPY --from=hard-builder /tmp/hard-source/format/format.v1 /usr/local/libexec/hard/format/format.v1
COPY --from=hard-builder /tmp/hard-source/LICENSE /usr/local/libexec/hard/licenses/hard-LICENSE
COPY --from=hard-builder /tmp/hard-version /usr/local/libexec/hard/VERSION

RUN ln -s /usr/bin/clang-format-18 /usr/local/libexec/hard/bin/clang-format \
    && test "$(cat /usr/local/libexec/hard/VERSION)" = "${HARD_VERSION}" \
    && test "$(getconf GNU_LIBC_VERSION)" = 'glibc 2.35' \
    && /usr/local/libexec/hard/hard --help >/dev/null

ENV PATH=/usr/local/libexec/hard/bin:$PATH
ENV HOME=/tmp
ENV HARD_ROOT=/hard
ENV HARD_ENV=linux64:v4.0-glibc.2.35
ENV HARD_CC=c++
ENV HARD_CFLAGS="-std=c++20 -march=x86-64-v3 -mtune=generic -O3 -flto=auto -Wall -Wextra"
ENV HARD_LDFLAGS="-std=c++20 -O3 -flto=auto -Wall -Wextra -static-libgcc -static-libstdc++"
ENV HARD_ENTRYPOINTS="main _start"

WORKDIR /project
ENTRYPOINT ["/usr/local/libexec/hard/hard"]
