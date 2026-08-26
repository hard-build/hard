FROM ubuntu:22.04

ARG DEBIAN_FRONTEND=noninteractive
ARG HARD_VERSION=v2.0
ARG HARD_SHA256=da51adc54d56219e427f198e610036b8c42d0306abfcfec58ea2c60033f42200
ARG HARD_REVISION=100406872f99fd4fcdb23425d21f638d58368237

LABEL org.opencontainers.image.source="https://github.com/hard-build/hard" \
      org.opencontainers.image.description="hard linux64 C++ build environment" \
      org.opencontainers.image.version="v2.0-ubuntu.22.04" \
      org.opencontainers.image.revision="${HARD_REVISION}"

RUN apt-get update \
    && apt-get install -y --no-install-recommends \
        autoconf \
        automake \
        build-essential \
        ca-certificates \
        cmake \
        curl \
        git \
        libgmock-dev \
        libgtest-dev \
        libtool \
        libtool-bin \
        meson \
        ninja-build \
        pkg-config \
    && rm -rf /var/lib/apt/lists/*

RUN curl --fail --location --retry 3 \
        --output /tmp/hard.tar.gz \
        "https://github.com/hard-build/hard/releases/download/${HARD_VERSION}/hard-${HARD_VERSION}.tar.gz" \
    && echo "${HARD_SHA256}  /tmp/hard.tar.gz" | sha256sum -c - \
    && tar -xzf /tmp/hard.tar.gz --directory /tmp \
    && install -d /usr/local/libexec \
    && cp -a /tmp/hard-linux-amd64/libexec/hard /usr/local/libexec/hard \
    && test "$(cat /usr/local/libexec/hard/VERSION)" = "${HARD_VERSION}" \
    && /usr/local/libexec/hard/hard --help >/dev/null \
    && rm -rf /tmp/hard.tar.gz /tmp/hard-linux-amd64

RUN install -d /hard /project \
    && chmod 0777 /hard /project

ENV PATH=/usr/local/libexec/hard/bin:$PATH
ENV HOME=/tmp
ENV HARD_ROOT=/hard
ENV HARD_ENV=linux64:v2.0-ubuntu.22.04
ENV HARD_CC=c++
ENV HARD_CFLAGS="-std=c++20 -march=x86-64-v3 -mtune=generic -O3 -flto=auto -Wall -Wextra -idirafter /usr/local/libexec/hard/lib/clang/18/include"
ENV HARD_LDFLAGS="-std=c++20 -O3 -flto=auto -Wall -Wextra -static-libgcc -static-libstdc++"
ENV HARD_ENTRYPOINTS="main _start"

RUN printf 'int main() {}\n' > /tmp/hard-smoke.cpp \
    && cd /tmp \
    && /usr/local/libexec/hard/hard build -s hard-smoke.cpp \
    && test -x /tmp/hard-smoke \
    && rm -rf /hard/env /hard/source /tmp/hard-smoke /tmp/hard-smoke.cpp

WORKDIR /project
ENTRYPOINT ["/usr/local/libexec/hard/hard"]
