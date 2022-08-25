clean:
	rm -rf ./bin

build-api:
	go build -ldflags="-s -w" -o bin/api cmd/api/main.go

test:
	go test ./...

test-verbose:
	go test ./... -v

run:
	air