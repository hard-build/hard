FROM alpine:3.22 AS hard-builder

ARG HARD_VERSION=v4.0
ARG HARD_REVISION

RUN apk add --no-cache \
        clang18-dev \
        curl \
        g++ \
        go \
    && install -d /usr/lib/llvm-18 \
    && ln -s /usr/lib/llvm18/include /usr/lib/llvm-18/include \
    && ln -s /usr/lib/libclang.so /usr/lib/libclang-18.so

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

FROM alpine:3.22

ARG HARD_VERSION=v4.0
ARG HARD_REVISION

LABEL org.opencontainers.image.source="https://github.com/hard-build/hard" \
      org.opencontainers.image.description="hard linux64 fully static C++ build environment with musl 1.2.5" \
      org.opencontainers.image.version="v4.0-musl.1.2.5-static" \
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
COPY --from=hard-builder /tmp/hard-source/hard.h /usr/local/libexec/hard/hard.h
COPY --from=hard-builder /tmp/hard-source/format/format.v1 /usr/local/libexec/hard/format/format.v1
COPY --from=hard-builder /tmp/hard-source/LICENSE /usr/local/libexec/hard/licenses/hard-LICENSE
COPY --from=hard-builder /tmp/hard-version /usr/local/libexec/hard/VERSION

RUN ln -s /usr/lib/llvm18/bin/clang-format /usr/local/libexec/hard/bin/clang-format \
    && test "$(cat /usr/local/libexec/hard/VERSION)" = "${HARD_VERSION}" \
    && ldd --version 2>&1 | grep -F 'Version 1.2.5' \
    && /usr/local/libexec/hard/hard --help >/dev/null

ENV PATH=/usr/local/libexec/hard/bin:$PATH
ENV HOME=/tmp
ENV HARD_ROOT=/hard
ENV HARD_ENV=linux64:v4.0-musl.1.2.5-static
ENV HARD_CC=c++
ENV HARD_CFLAGS="-std=c++20 -march=x86-64-v3 -mtune=generic -O3 -flto=auto -Wall -Wextra"
ENV HARD_LDFLAGS="-std=c++20 -O3 -flto=auto -Wall -Wextra -static -static-libgcc -static-libstdc++"
ENV HARD_ENTRYPOINTS="main _start"

WORKDIR /project
ENTRYPOINT ["/usr/local/libexec/hard/hard"]
