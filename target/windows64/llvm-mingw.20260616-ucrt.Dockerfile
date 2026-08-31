FROM golang:1.23.12-bookworm AS go

FROM ubuntu:22.04 AS llvm22-base

ARG DEBIAN_FRONTEND=noninteractive
ARG LLVM_PACKAGE_VERSION=1:22.1.8~++20260613092327+e80beda6e255-1~exp1~20260613092437.81

RUN apt-get update \
    && apt-get install -y --no-install-recommends \
        ca-certificates \
        curl \
        gnupg \
        xz-utils \
    && curl --fail --location --retry 3 \
        --output /tmp/llvm-snapshot.gpg.key \
        https://apt.llvm.org/llvm-snapshot.gpg.key \
    && gpg --dearmor \
        --output /usr/share/keyrings/apt.llvm.org.gpg \
        /tmp/llvm-snapshot.gpg.key \
    && printf '%s\n' \
        'deb [signed-by=/usr/share/keyrings/apt.llvm.org.gpg] https://apt.llvm.org/jammy/ llvm-toolchain-jammy-22 main' \
        > /etc/apt/sources.list.d/apt.llvm.org.list \
    && rm -f /tmp/llvm-snapshot.gpg.key \
    && rm -rf /var/lib/apt/lists/*

FROM llvm22-base AS hard-builder

ARG HARD_VERSION
ARG HARD_REVISION
ARG IMAGE_VERSION
ARG LLVM_PACKAGE_VERSION=1:22.1.8~++20260613092327+e80beda6e255-1~exp1~20260613092437.81

COPY --from=go /usr/local/go /usr/local/go

ENV PATH=/usr/local/go/bin:$PATH

RUN apt-get update \
    && apt-get install -y --no-install-recommends \
        build-essential \
        libclang-22-dev="${LLVM_PACKAGE_VERSION}" \
    && rm -rf /var/lib/apt/lists/* \
    && install -d /usr/lib/llvm-18 \
    && ln -s /usr/lib/llvm-22/include /usr/lib/llvm-18/include \
    && ln -s /usr/lib/llvm-22/lib/libclang.so /usr/lib/x86_64-linux-gnu/libclang-18.so

COPY hard /tmp/hard-source/hard
COPY hard.h /tmp/hard-source/hard.h
COPY format /tmp/hard-source/format
COPY LICENSE /tmp/hard-source/LICENSE
COPY unittest/requirements.txt /tmp/hard-source/unittest/requirements.txt

RUN test -n "${HARD_VERSION}" \
    && test -n "${HARD_REVISION}" \
    && cd /tmp/hard-source/hard \
    && CGO_ENABLED=1 go build \
        -trimpath \
        -ldflags="-s -w -X main.versionPrerelease=" \
        -o /tmp/hard . \
    && test "$(/tmp/hard version)" = "${HARD_VERSION}" \
    && /tmp/hard --help >/dev/null

FROM llvm22-base

ARG DEBIAN_FRONTEND=noninteractive
ARG HARD_VERSION
ARG HARD_REVISION
ARG IMAGE_VERSION
ARG LLVM_MINGW_SHA256=534b92e067b22a6b4441f48ae9240a3341b17825d04d577eab0cf85c44b4deda
ARG LLVM_MINGW_VERSION=20260616
ARG LLVM_PACKAGE_VERSION=1:22.1.8~++20260613092327+e80beda6e255-1~exp1~20260613092437.81

LABEL org.opencontainers.image.source="https://github.com/hard-build/hard" \
      org.opencontainers.image.description="hard windows64 C++ cross-build environment with LLVM-MinGW, UCRT, and Wine" \
      org.opencontainers.image.version="${IMAGE_VERSION}" \
      org.opencontainers.image.revision="${HARD_REVISION}"

RUN apt-get update \
    && apt-get install -y --no-install-recommends \
        autoconf \
        automake \
        cmake \
        git \
        libclang1-22="${LLVM_PACKAGE_VERSION}" \
        libgtest-dev \
        libtool \
        libtool-bin \
        make \
        meson \
        ninja-build \
        pkg-config \
        python3-pip \
        wine64 \
    && rm -rf /var/lib/apt/lists/* \
    && ln -s /usr/lib/wine/wine64 /usr/local/bin/wine

RUN curl --fail --location --retry 3 \
        --output /tmp/llvm-mingw.tar.xz \
        "https://github.com/mstorsjo/llvm-mingw/releases/download/${LLVM_MINGW_VERSION}/llvm-mingw-${LLVM_MINGW_VERSION}-ucrt-ubuntu-22.04-x86_64.tar.xz" \
    && echo "${LLVM_MINGW_SHA256}  /tmp/llvm-mingw.tar.xz" | sha256sum -c - \
    && tar -xJf /tmp/llvm-mingw.tar.xz --directory /opt \
    && mv \
        "/opt/llvm-mingw-${LLVM_MINGW_VERSION}-ucrt-ubuntu-22.04-x86_64" \
        /opt/llvm-mingw \
    && rm -f /tmp/llvm-mingw.tar.xz \
    && /opt/llvm-mingw/bin/x86_64-w64-mingw32-clang++ --version \
        | grep -F 'clang version 22.1.8'

RUN install -d /opt/windows64 \
    && printf '%s\n' \
        'set(CMAKE_SYSTEM_NAME Windows)' \
        'set(CMAKE_SYSTEM_PROCESSOR x86_64)' \
        'set(CMAKE_C_COMPILER /opt/llvm-mingw/bin/x86_64-w64-mingw32-clang)' \
        'set(CMAKE_CXX_COMPILER /opt/llvm-mingw/bin/x86_64-w64-mingw32-clang++)' \
        'set(CMAKE_RC_COMPILER /opt/llvm-mingw/bin/x86_64-w64-mingw32-windres)' \
        'set(CMAKE_CROSSCOMPILING_EMULATOR /usr/local/bin/wine)' \
        'set(CMAKE_FIND_ROOT_PATH /opt/llvm-mingw/x86_64-w64-mingw32 /opt/windows64)' \
        'set(CMAKE_FIND_ROOT_PATH_MODE_PROGRAM NEVER)' \
        'set(CMAKE_FIND_ROOT_PATH_MODE_LIBRARY ONLY)' \
        'set(CMAKE_FIND_ROOT_PATH_MODE_INCLUDE ONLY)' \
        'set(CMAKE_FIND_ROOT_PATH_MODE_PACKAGE ONLY)' \
        > /opt/windows64/toolchain.cmake \
    && cmake \
        -S /usr/src/googletest \
        -B /tmp/googletest-build \
        -DCMAKE_BUILD_TYPE=Release \
        -DCMAKE_INSTALL_PREFIX=/opt/windows64 \
        -DCMAKE_TOOLCHAIN_FILE=/opt/windows64/toolchain.cmake \
        -DBUILD_GMOCK=ON \
        -DBUILD_SHARED_LIBS=OFF \
        -DINSTALL_GTEST=ON \
    && cmake --build /tmp/googletest-build --parallel \
    && cmake --install /tmp/googletest-build \
    && rm -rf /tmp/googletest-build

RUN install -d \
        /hard \
        /project \
        /usr/local/libexec/hard/bin \
        /usr/local/libexec/hard/format \
        /usr/local/libexec/hard/lib/clang \
        /usr/local/libexec/hard/licenses \
    && chmod 0777 /hard /project

COPY --from=hard-builder /tmp/hard /usr/local/libexec/hard/hard
COPY --from=hard-builder /tmp/hard-source/hard.h /usr/local/libexec/hard/hard.h
COPY --from=hard-builder /tmp/hard-source/format/format.v1 /usr/local/libexec/hard/format/format.v1
COPY --from=hard-builder /tmp/hard-source/LICENSE /usr/local/libexec/hard/licenses/hard-LICENSE
COPY --from=hard-builder /tmp/hard-source/unittest/requirements.txt /tmp/hard-unittest-requirements.txt

RUN python3 -m pip install \
        --no-cache-dir \
        --requirement /tmp/hard-unittest-requirements.txt \
    && rm -f /tmp/hard-unittest-requirements.txt \
    && ln -s /opt/llvm-mingw/bin/clang-format /usr/local/libexec/hard/bin/clang-format \
    && ln -s /opt/llvm-mingw/lib/clang/22 /usr/local/libexec/hard/lib/clang/22 \
    && test -n "${HARD_VERSION}" \
    && test -n "${HARD_REVISION}" \
    && test "${IMAGE_VERSION}" = "${HARD_VERSION}-llvm-mingw.20260616-ucrt" \
    && test "$(/usr/local/libexec/hard/hard version)" = "${HARD_VERSION}" \
    && /usr/local/libexec/hard/hard --help >/dev/null

ENV PATH=/usr/local/libexec/hard/bin:/opt/llvm-mingw/bin:$PATH
ENV HOME=/tmp
ENV HARD_ROOT=/hard
ENV HARD_ENV=windows64:${IMAGE_VERSION}
ENV HARD_CC=x86_64-w64-mingw32-clang++
ENV HARD_CFLAGS="-std=c++20 --target=x86_64-w64-mingw32 --sysroot=/opt/llvm-mingw/x86_64-w64-mingw32 -stdlib=libc++ -march=x86-64-v3 -mtune=generic -O3 -flto=auto -Wall -Wextra"
ENV HARD_LDFLAGS="-std=c++20 --target=x86_64-w64-mingw32 --sysroot=/opt/llvm-mingw/x86_64-w64-mingw32 -stdlib=libc++ -march=x86-64-v3 -mtune=generic -O3 -flto=auto -Wall -Wextra -static"
ENV HARD_ENTRYPOINTS="main _start"
ENV HARD_EXECUTABLE_SUFFIX=.exe
ENV HARD_EXECUTABLE_RUNNER=wine
ENV CMAKE_TOOLCHAIN_FILE=/opt/windows64/toolchain.cmake
ENV PKG_CONFIG_LIBDIR=/opt/windows64/lib/pkgconfig
ENV WINEARCH=win64
ENV WINEDEBUG=-all
ENV WINEPREFIX=/hard/env/windows64:${IMAGE_VERSION}/wine

WORKDIR /project
ENTRYPOINT ["/usr/local/libexec/hard/hard"]
