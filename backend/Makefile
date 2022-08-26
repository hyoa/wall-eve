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

build-cli:
	GOOS=windows GOARCH=amd64 go build -o bin/wall-eve-cli.exe cmd/cli/main.go
	GOOS=darwin GOARCH=amd64 go build -o bin/wall-eve-cli-macos cmd/cli/main.go
	GOOS=linux GOARCH=amd64 go build -o bin/wall-eve-cli-linux cmd/cli/main.go