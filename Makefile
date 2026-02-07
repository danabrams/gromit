.PHONY: build install install-skill
build:
	go build -o gromit ./cmd/gromit
	go install ./cmd/gromit

install:
	go install ./cmd/gromit

install-skill: build
	./gromit install-skill


