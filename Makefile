build:
	rm -rf bin/
	go build -o bin/issuer ./cmd/issuer
	go build -o bin/server ./cmd/server