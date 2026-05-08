.PHONY: build clean test run

build:
	go build -o jobpilot ./cmd/srv

clean:
	rm -f jobpilot

test:
	go test ./...

run: build
	./jobpilot
