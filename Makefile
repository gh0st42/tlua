BINARY := bin/tlua
PKG    := ./cmd/tlua

.PHONY: all build static test test-gui fmt vet clean
all: build

build:
	go build -o $(BINARY) $(PKG)

# A fully static binary; no cgo anywhere in the tree.
static:
	CGO_ENABLED=0 go build -ldflags '-s -w' -o $(BINARY) $(PKG)

test:
	go test ./...

# The GUI tests open windows and drive them with synthetic input, so they
# are kept out of plain `make test`. Typing elsewhere while they run can
# get in their way.
test-gui:
	TLUA_GUI_TESTS=1 go test -count=1 ./internal/gui ./internal/design

fmt:
	gofmt -w .

vet:
	go vet ./...

clean:
	rm -rf bin
