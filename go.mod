module server

// +heroku goVersion go1.14
go 1.14

// +heroku install ./cmd/issuer

require github.com/lib/pq v1.9.0
