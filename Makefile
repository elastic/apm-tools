GO_TEST_TIMEOUT=30s

fmt: tools/go.mod
	@go run -modfile=tools/go.mod github.com/elastic/go-licenser .
	@go run -modfile=tools/go.mod golang.org/x/tools/cmd/goimports -local github.com/elastic/ -w .

lint: tools/go.mod
	go run -modfile=tools/go.mod honnef.co/go/tools/cmd/staticcheck -checks=all ./...
	go mod tidy && git diff --exit-code

.PHONY: test
test: go.mod
	go test -race -v -timeout=$(GO_TEST_TIMEOUT) ./...

.PHONY: integration-test
integration-test:
	 go test --tags=integration ./...
