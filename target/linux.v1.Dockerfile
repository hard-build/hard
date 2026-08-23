FROM ubuntu:22.04 AS builder

ARG DEBIAN_FRONTEND=noninteractive
ARG GO_VERSION=1.23.12
ARG GO_SHA256=d3847fef834e9db11bf64e3fb34db9c04db14e068eeb064f49af747010454f90

RUN apt-get update \
    && apt-get install -y --no-install-recommends \
        ca-certificates \
        curl \
    && curl -fsSLo /usr/share/keyrings/apt.llvm.org.asc https://apt.llvm.org/llvm-snapshot.gpg.key \
    && echo "deb [signed-by=/usr/share/keyrings/apt.llvm.org.asc] https://apt.llvm.org/jammy/ llvm-toolchain-jammy-18 main" > /etc/apt/sources.list.d/apt.llvm.org.list \
    && apt-get update \
    && apt-get install -y --no-install-recommends \
        build-essential \
        libclang-18-dev \
    && rm -rf /var/lib/apt/lists/*

RUN curl -fsSLo /tmp/go.tar.gz "https://go.dev/dl/go${GO_VERSION}.linux-amd64.tar.gz" \
    && echo "${GO_SHA256}  /tmp/go.tar.gz" | sha256sum -c - \
    && tar -C /usr/local -xzf /tmp/go.tar.gz \
    && rm /tmp/go.tar.gz

ENV PATH=/usr/local/go/bin:$PATH

WORKDIR /src/hard
COPY hard/go.mod hard/go.sum ./
RUN go mod download
COPY hard/ ./
RUN CGO_ENABLED=1 GOOS=linux GOARCH=amd64 go build -trimpath -ldflags="-s -w" -o /out/hard .

FROM ubuntu:22.04

ARG DEBIAN_FRONTEND=noninteractive

LABEL org.opencontainers.image.source="https://github.com/hard-build/hard" \
      org.opencontainers.image.description="hard linux.v1 C++ build environment" \
      org.opencontainers.image.version="linux.v1"

RUN apt-get update \
    && apt-get install -y --no-install-recommends \
        ca-certificates \
        curl \
    && curl -fsSLo /usr/share/keyrings/apt.llvm.org.asc https://apt.llvm.org/llvm-snapshot.gpg.key \
    && echo "deb [signed-by=/usr/share/keyrings/apt.llvm.org.asc] https://apt.llvm.org/jammy/ llvm-toolchain-jammy-18 main" > /etc/apt/sources.list.d/apt.llvm.org.list \
    && apt-get update \
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
    && ln -s /usr/bin/clang-format-18 /usr/local/bin/clang-format \
    && rm -rf /var/lib/apt/lists/*

RUN install -d /usr/local/libexec/hard/format /hard /project \
    && chmod 0777 /hard /project

COPY --from=builder /out/hard /usr/local/libexec/hard/hard
COPY hard.h /usr/local/libexec/hard/hard.h
COPY format/format.v1 /usr/local/libexec/hard/format/format.v1

ENV HOME=/tmp
ENV HARD_ROOT=/hard
ENV HARD_ENV=linux.v1
ENV HARD_CC=c++
ENV HARD_CFLAGS="-std=c++20 -march=x86-64-v3 -mtune=generic -O3 -flto=auto -Wall -Wextra"
ENV HARD_LDFLAGS="-std=c++20 -O3 -flto=auto -Wall -Wextra -static-libgcc -static-libstdc++"
ENV HARD_ENTRYPOINTS="main _start"

WORKDIR /project
ENTRYPOINT ["/usr/local/libexec/hard/hard"]
