.PHONY: build install
build:
	go build -o gromit ./cmd/gromit
	go install ./cmd/gromit

install:
	go install ./cmd/gromit


