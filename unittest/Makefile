PYTHON ?= python3
HARD ?= hard
OUTPUT ?= /tmp/hard-unittest
JOBS ?= 0
SCENARIO ?=

.PHONY: all check

all: check

check:
	"$(PYTHON)" run.py --hard "$(HARD)" --output "$(OUTPUT)" \
		--jobs "$(JOBS)" $(if $(SCENARIO),"$(SCENARIO)")
