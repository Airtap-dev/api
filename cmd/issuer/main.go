package main

import (
	"flag"
	"log"
	"os"

	"database/sql"

	"github.com/golang-migrate/migrate/v4"
	"github.com/golang-migrate/migrate/v4/database/postgres"
	_ "github.com/golang-migrate/migrate/v4/source/file"
	_ "github.com/lib/pq"
)

var dbGlobal *sql.DB

func init() {
	// Connect.
	if db, err := sql.Open("postgres", os.Getenv("DATABASE_URL")); err != nil {
		log.Panic(err)
	} else {
		dbGlobal = db
	}

	// Ping.
	if err := dbGlobal.Ping(); err != nil {
		log.Panic(err)
	}

	// Make sure the tables are in place.
	if driver, err := postgres.WithInstance(dbGlobal, &postgres.Config{}); err != nil {
		log.Panic(err)
	} else if m, err := migrate.NewWithDatabaseInstance("file://migrations", "postgres", driver); err != nil {
		log.Panic(err)
	} else {
		m.Up()
	}
}

func main() {
	max := flag.Int("max", 10, "maximum number of activations for this license")
	flag.Parse()

	rows, err := dbGlobal.Query(issueQuery, *max)
	if err != nil {
		log.Panic(err)
	}

	defer rows.Close()

	var (
		license        string
		maxActivations int
		revoked        bool
	)

	for rows.Next() {
		if err := rows.Scan(&license, &maxActivations, &revoked); err != nil {
			log.Panic(err)
		} else {
			if !revoked {
				log.Printf("Created new license: %v with %v maximum activations. Currently not revoked.", license, maxActivations)
			} else {
				log.Printf("Created new license: %v with %v maximum activations. Currently revoked.", license, maxActivations)
			}
		}
	}

	err = rows.Err()
	if err != nil {
		log.Panic(err)
	}

	dbGlobal.Close()
}
