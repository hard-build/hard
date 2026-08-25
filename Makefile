GO ?= go
INSTALL ?= install
PREFIX ?= $(HOME)/.local
DESTDIR ?=
BUILD_DIR ?= build

HARD_BINARY := $(abspath $(BUILD_DIR)/hard)
COMPLETION_BUILD_DIR := $(abspath $(BUILD_DIR)/completion)
BINDIR := $(PREFIX)/bin
LIBEXECDIR := $(PREFIX)/libexec/hard
HARD_ROOT := $(PREFIX)/share/hard
BASH_COMPLETIONDIR := $(PREFIX)/share/bash-completion/completions
ZSH_COMPLETIONDIR := $(PREFIX)/share/zsh/site-functions
FISH_COMPLETIONDIR := $(PREFIX)/share/fish/vendor_completions.d

.PHONY: all build install

all: build

build:
	$(INSTALL) -d "$(dir $(HARD_BINARY))"
	cd hard && $(GO) build -o "$(HARD_BINARY)" .

install: build
	$(INSTALL) -d "$(COMPLETION_BUILD_DIR)"
	"$(HARD_BINARY)" completion bash > "$(COMPLETION_BUILD_DIR)/hard"
	"$(HARD_BINARY)" completion zsh > "$(COMPLETION_BUILD_DIR)/_hard"
	"$(HARD_BINARY)" completion fish > "$(COMPLETION_BUILD_DIR)/hard.fish"
	$(INSTALL) -d "$(DESTDIR)$(BINDIR)"
	$(INSTALL) -d "$(DESTDIR)$(LIBEXECDIR)"
	$(INSTALL) -d "$(DESTDIR)$(LIBEXECDIR)/format"
	$(INSTALL) -d "$(DESTDIR)$(HARD_ROOT)"
	$(INSTALL) -d "$(DESTDIR)$(BASH_COMPLETIONDIR)"
	$(INSTALL) -d "$(DESTDIR)$(ZSH_COMPLETIONDIR)"
	$(INSTALL) -d "$(DESTDIR)$(FISH_COMPLETIONDIR)"
	$(INSTALL) -m 0755 hard.sh "$(DESTDIR)$(BINDIR)/hard"
	$(INSTALL) -m 0755 "$(HARD_BINARY)" "$(DESTDIR)$(LIBEXECDIR)/hard"
	$(INSTALL) -m 0644 hard.h "$(DESTDIR)$(LIBEXECDIR)/hard.h"
	$(INSTALL) -m 0644 format/format.v1 "$(DESTDIR)$(LIBEXECDIR)/format/format.v1"
	$(INSTALL) -m 0644 "$(COMPLETION_BUILD_DIR)/hard" "$(DESTDIR)$(BASH_COMPLETIONDIR)/hard"
	$(INSTALL) -m 0644 "$(COMPLETION_BUILD_DIR)/_hard" "$(DESTDIR)$(ZSH_COMPLETIONDIR)/_hard"
	$(INSTALL) -m 0644 "$(COMPLETION_BUILD_DIR)/hard.fish" "$(DESTDIR)$(FISH_COMPLETIONDIR)/hard.fish"
	$(INSTALL) -m 0644 /dev/null "$(DESTDIR)$(LIBEXECDIR)/default-target"
	printf 'host\n' > "$(DESTDIR)$(LIBEXECDIR)/default-target"
