vet:
	go vet ./...

fmt:
	go fmt ./...

lint:
	golangci-lint run

test:
	go test -v ./...

build:
	go build -o bin/peggbot cmd/peggbot/main.go

run:
	./bin/peggbot

clean:
	rm -rf bin