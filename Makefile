fmt: tools/go.mod
	@go run -modfile=tools/go.mod github.com/elastic/go-licenser .
	@go run -modfile=tools/go.mod golang.org/x/tools/cmd/goimports -local github.com/elastic/ -w .

lint: tools/go.mod
	go run -modfile=tools/go.mod honnef.co/go/tools/cmd/staticcheck -checks=all ./...
	go mod tidy && git diff --exit-code
