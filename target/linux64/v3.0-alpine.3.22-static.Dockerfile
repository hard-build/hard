FROM alpine:3.22 AS hard-builder

ARG HARD_VERSION=v3.0
ARG HARD_SOURCE_SHA256=ee24cbeec82087f31a0c07d7a346f85c0f3d5b36fd25199a90ebb69c1e1bee35

RUN apk add --no-cache \
        clang18-dev \
        curl \
        g++ \
        go \
    && install -d /usr/lib/llvm-18 \
    && ln -s /usr/lib/llvm18/include /usr/lib/llvm-18/include \
    && ln -s /usr/lib/libclang.so /usr/lib/libclang-18.so

RUN curl --fail --location --retry 3 \
        --output /tmp/hard.tar.gz \
        "https://github.com/hard-build/hard/archive/refs/tags/${HARD_VERSION}.tar.gz" \
    && echo "${HARD_SOURCE_SHA256}  /tmp/hard.tar.gz" | sha256sum -c - \
    && tar -xzf /tmp/hard.tar.gz --directory /tmp \
    && cd "/tmp/hard-${HARD_VERSION#v}/hard" \
    && CGO_ENABLED=1 go build \
        -trimpath \
        -ldflags="-s -w" \
        -o /tmp/hard .

FROM alpine:3.22

ARG HARD_VERSION=v3.0
ARG HARD_REVISION=3826020ccc617f189521e5628e2ce5f8ecf82e00

LABEL org.opencontainers.image.source="https://github.com/hard-build/hard" \
      org.opencontainers.image.description="hard linux64 static C++ build environment based on Alpine Linux" \
      org.opencontainers.image.version="v3.0-alpine.3.22-static" \
      org.opencontainers.image.revision="${HARD_REVISION}"

RUN apk add --no-cache \
        autoconf \
        automake \
        build-base \
        ca-certificates \
        clang18-dev \
        clang18-extra-tools \
        cmake \
        curl \
        git \
        gtest-dev \
        gtest-src \
        libtool \
        meson \
        pkgconf \
        samurai

RUN c++ -std=c++20 -pthread -I/usr/src/gtest -c \
        /usr/src/gtest/src/gtest-all.cc -o /tmp/gtest-all.o \
    && ar rcs /usr/lib/libgtest.a /tmp/gtest-all.o \
    && c++ -std=c++20 -pthread -I/usr/src/gtest -c \
        /usr/src/gtest/src/gtest_main.cc -o /tmp/gtest-main.o \
    && ar rcs /usr/lib/libgtest_main.a /tmp/gtest-main.o \
    && rm /tmp/gtest-all.o /tmp/gtest-main.o

RUN install -d \
        /hard \
        /project \
        /usr/local/libexec/hard/bin \
        /usr/local/libexec/hard/format \
        /usr/local/libexec/hard/licenses \
    && chmod 0777 /hard /project

COPY --from=hard-builder /tmp/hard /usr/local/libexec/hard/hard
COPY --from=hard-builder /tmp/hard-3.0/hard.h /usr/local/libexec/hard/hard.h
COPY --from=hard-builder /tmp/hard-3.0/format/format.v1 /usr/local/libexec/hard/format/format.v1
COPY --from=hard-builder /tmp/hard-3.0/LICENSE /usr/local/libexec/hard/licenses/hard-LICENSE

RUN ln -s /usr/lib/llvm18/bin/clang-format /usr/local/libexec/hard/bin/clang-format \
    && printf '%s\n' "${HARD_VERSION}" > /usr/local/libexec/hard/VERSION \
    && test "$(cat /usr/local/libexec/hard/VERSION)" = "${HARD_VERSION}" \
    && /usr/local/libexec/hard/hard --help >/dev/null

ENV PATH=/usr/local/libexec/hard/bin:$PATH
ENV HOME=/tmp
ENV HARD_ROOT=/hard
ENV HARD_ENV=linux64:v3.0-alpine.3.22-static
ENV HARD_CC=c++
ENV HARD_CFLAGS="-std=c++20 -march=x86-64-v3 -mtune=generic -O3 -flto=auto -Wall -Wextra"
ENV HARD_LDFLAGS="-std=c++20 -O3 -flto=auto -Wall -Wextra -static -static-libgcc -static-libstdc++"
ENV HARD_ENTRYPOINTS="main _start"

WORKDIR /project
ENTRYPOINT ["/usr/local/libexec/hard/hard"]
