GO ?= go
INSTALL ?= install
PREFIX ?= $(HOME)/.local
DESTDIR ?=
BUILD_DIR ?= build

HARD_BINARY := $(abspath $(BUILD_DIR)/hard)
BINDIR := $(PREFIX)/bin
LIBEXECDIR := $(PREFIX)/libexec/hard
HARD_ROOT := $(PREFIX)/share/hard

.PHONY: all build install

all: build

build:
	$(INSTALL) -d "$(dir $(HARD_BINARY))"
	cd hard && $(GO) build -o "$(HARD_BINARY)" .

install: build
	$(INSTALL) -d "$(DESTDIR)$(BINDIR)"
	$(INSTALL) -d "$(DESTDIR)$(LIBEXECDIR)"
	$(INSTALL) -d "$(DESTDIR)$(LIBEXECDIR)/format"
	$(INSTALL) -d "$(DESTDIR)$(HARD_ROOT)"
	$(INSTALL) -m 0755 hard.sh "$(DESTDIR)$(BINDIR)/hard"
	$(INSTALL) -m 0755 "$(HARD_BINARY)" "$(DESTDIR)$(LIBEXECDIR)/hard"
	$(INSTALL) -m 0644 hard.h "$(DESTDIR)$(LIBEXECDIR)/hard.h"
	$(INSTALL) -m 0644 format/format.v1 "$(DESTDIR)$(LIBEXECDIR)/format/format.v1"
