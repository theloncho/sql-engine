.PHONY: all build test clean run

# Binary name
BINARY_NAME=sql-engine.exe

all: test build

build:
	go build -o $(BINARY_NAME) .

test:
	go test -v ./...

run: build
	./$(BINARY_NAME) -load data/employees.csv,data/departments.csv

clean:
	go clean
	rm -f $(BINARY_NAME)
