.PHONY: build install
build:
	go build -o ralph ./cmd/ralph
	go install ./cmd/ralph

install:
	go install ./cmd/ralph


