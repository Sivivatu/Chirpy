.PHONY: build run dev

build:
	go build -o out

run:
	./out

dev: build run
