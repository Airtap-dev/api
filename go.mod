module server

// +heroku goVersion go1.14
go 1.14

// +heroku install ./cmd/issuer ./cmd/api

require (
	github.com/golang-migrate/migrate/v4 v4.14.1
	github.com/gorilla/websocket v1.4.2
	github.com/lib/pq v1.9.0
)
