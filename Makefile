.PHONY: build run dev test

build:
	go build

test:
	go test ./...

run:
	./chirpy

dev: build test run
