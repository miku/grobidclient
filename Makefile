SHELL := /bin/bash
TARGETS := grobidcli

.PHONY: test
test:
	go test -v -cover ./...

.PHONY: all
all: $(TARGETS)

%: cmd/%/main.go
	go build -o $@ $<

.PHONY: clean
clean:
	rm -f $(TARGETS)

.PHONY: update-all-deps
update-all-deps:
	go get -u -v ./... && go mod tidy

