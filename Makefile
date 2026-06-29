.PHONY: build run dry-run tidy

build:
	go build -o chaos-sloth .

run:
	go run . -config config.yaml

dry-run:
	go run . -config config.yaml

tidy:
	go mod tidy
