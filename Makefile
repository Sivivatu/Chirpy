.PHONY: build run dev test

build:
	go build

test:
	go test -v ./...

run:
	./chirpy

dev:
	@go build
	@go test ./...
	@./chirpy
