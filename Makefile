BINARY := solar-moonitor
PKG := ./cmd/solar-moonitor

.PHONY: build run demo test race check fmt vet tidy clean discover install

build:
	go build -trimpath -o $(BINARY) $(PKG)

run:
	go run $(PKG)

demo:
	go run $(PKG) --demo

test:
	go test ./...

race:
	go test -race ./...

fmt:
	gofmt -w $$(find . -name '*.go' -not -path './.git/*')

vet:
	go vet ./...

check:
	@test -z "$$(gofmt -l $$(find . -name '*.go' -not -path './.git/*'))" || \
		(echo "gofmt needed:"; gofmt -l $$(find . -name '*.go' -not -path './.git/*'); exit 1)
	go vet ./...
	go test ./...

tidy:
	go mod tidy

clean:
	rm -f $(BINARY)

discover:
	go run $(PKG) --discover

install:
	go install $(PKG)
