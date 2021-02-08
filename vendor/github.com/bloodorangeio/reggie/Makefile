.PHONY: test
test:
	go test -v -race -cover -coverprofile=coverage.out -covermode=atomic ./...

.PHONY: covhtml
covhtml:
	go tool cover -html=coverage.out -o coverage.html
