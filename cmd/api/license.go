package main

import (
	"database/sql"
	"log"
)

func checkLicense(license string) (bool, int, error) {
	row := dbGlobal.QueryRow(findLicenseQuery, license)

	var id, maxActivations int
	var revoked bool
	if err := row.Scan(&id, &maxActivations, &revoked); err == sql.ErrNoRows {
		return false, 0, nil
	} else if err != nil {
		log.Print(err)
		return false, 0, errInternal
	}

	if revoked {
		return false, 0, nil
	}

	row = dbGlobal.QueryRow(findLicenseUsers, id)
	var currentActivations int
	if err := row.Scan(&currentActivations); err != nil {
		log.Print(err)
		return false, 0, errInternal
	}

	if currentActivations >= maxActivations {
		return false, 0, nil
	}

	return true, id, nil
}
