.PHONY: check build smoke release install
VERSION ?= $(shell cat VERSION)
DENY_PATTERNS ?=
AUDIT_ARGS = $(if $(DENY_PATTERNS),--deny-patterns "$(DENY_PATTERNS)")
check:
	go test -race ./...
	node --test internal/extension/*.test.cjs examples/ttyd/*.test.cjs plugins/*.test.cjs
	@for source in internal/extension/*.js; do node --check "$$source" || exit; done
	python3 -B -m unittest discover -s scripts -p 'test_*.py'
	python3 scripts/audit.py $(AUDIT_ARGS)
build:
	mkdir -p bin
	go build -trimpath -ldflags "-X main.version=$(VERSION)" -o bin/bab ./cmd/bab
	go build -trimpath -o bin/bridge-demo ./cmd/bridge-demo
	go build -trimpath -ldflags "-X main.version=$(VERSION)" -o bin/bridge-ttyd ./cmd/bridge-ttyd
release:
	python3 scripts/build_release.py --version "$(VERSION)"

.PHONY: install-bab
PREFIX ?= $(HOME)/.local
install-bab:
	mkdir -p "$(PREFIX)/bin"
	go build -trimpath -ldflags "-X main.version=$(VERSION)" -o "$(PREFIX)/bin/bab" ./cmd/bab
	mkdir -p "$(PREFIX)/share/doc/bab"
	install -m 644 LICENSE NOTICE "$(PREFIX)/share/doc/bab/"
	install -m 644 "$$(go env GOROOT)/LICENSE" "$(PREFIX)/share/doc/bab/GO_LICENSE"
	"$(PREFIX)/bin/bab" completion zsh --install --prefix "$(PREFIX)"
	@echo 'Installed bab. Run bab setup (ensure the install directory is on PATH).'

.PHONY: install-ttyd
install-ttyd:
	mkdir -p "$(PREFIX)/bin"
	go build -trimpath -ldflags "-X main.version=$(VERSION)" -o "$(PREFIX)/bin/bridge-ttyd" ./cmd/bridge-ttyd

# Isolated local installation; never registers a Chrome Native Host.
smoke:
	python3 scripts/check_install.py

install: install-bab
