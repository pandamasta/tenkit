APP_NAME=tenkit
PKG=github.com/pandamasta/$(APP_NAME)

.PHONY: build run test fmt clean

build:
	go build -o bin/$(APP_NAME) ./example/

run:
	cd example && go run .

test:
	go test ./...

fmt:
	go fmt ./...

clean:
	rm -rf bin/
