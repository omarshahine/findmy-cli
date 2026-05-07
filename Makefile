SHELL := /bin/bash
BIN := bin
HELPER_SRC := helpers/findmy-helper/main.swift
HELPER_BIN := $(BIN)/findmy-helper
GO_BIN := $(BIN)/findmy

.PHONY: all build helper clean install

all: build

build: helper $(GO_BIN)

helper: $(HELPER_BIN)

$(HELPER_BIN): $(HELPER_SRC)
	@mkdir -p $(BIN)
	swiftc -O -o $@ $<

$(GO_BIN): $(shell find cmd internal -name '*.go') go.mod
	@mkdir -p $(BIN)
	go build -o $@ ./cmd/findmy

clean:
	rm -rf $(BIN)

install: build
	cp $(GO_BIN) $(HELPER_BIN) /usr/local/bin/
