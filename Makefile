build:
	rm -rf bin/
	go build -o bin/issuer ./cmd/issuer
	go build -o bin/api ./cmd/api

test:
	TEST=TEST TURN_SFO_KEY=secretkey go test ./...
	