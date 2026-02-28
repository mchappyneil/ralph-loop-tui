.PHONY: build lint clean

build:
	go build -o ralph-loop-tui

lint:
	golangci-lint run

clean:
	rm -f ralph-loop-tui
