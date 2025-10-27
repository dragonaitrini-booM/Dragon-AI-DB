BINARY=trini-pipeline

.PHONY: all build run clean docker-build

all: build

build:
	go build -o $(BINARY) main.go processor.go

run: build
	./$(BINARY)

clean:
	rm -f $(BINARY)
	rm -rf output/*.report.json

docker-build:
	docker build -t trini-pipeline:latest .
