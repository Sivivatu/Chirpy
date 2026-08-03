.PHONY: build run dev

build:
	go build

run:
	./chirpy

dev: build run
