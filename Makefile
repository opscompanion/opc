VERSION ?= $(shell git describe --tags --always --dirty 2>/dev/null || echo "dev")
COMMIT  ?= $(shell git rev-parse --short HEAD 2>/dev/null || echo "none")
DATE    ?= $(shell date -u +%Y-%m-%dT%H:%M:%SZ)
LDFLAGS  = -s -w \
           -X github.com/opscompanion/opc/cmd.Version=$(VERSION) \
           -X github.com/opscompanion/opc/cmd.Commit=$(COMMIT) \
           -X github.com/opscompanion/opc/cmd.Date=$(DATE)

.PHONY: build install clean release snapshot

build:
	go build -ldflags "$(LDFLAGS)" -o opc .

install:
	go install -ldflags "$(LDFLAGS)" .

clean:
	rm -f opc

snapshot:
	goreleaser release --snapshot --clean

release:
	goreleaser release --clean
