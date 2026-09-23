BINARY := bin/tlua
PKG    := ./cmd/tlua

.PHONY: all build static test fmt vet clean
all: build

build:
	go build -o $(BINARY) $(PKG)

# A fully static binary; no cgo anywhere in the tree.
static:
	CGO_ENABLED=0 go build -ldflags '-s -w' -o $(BINARY) $(PKG)

test:
	go test ./...

fmt:
	gofmt -w .

vet:
	go vet ./...

clean:
	rm -rf bin
