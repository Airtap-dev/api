package main

const findLicenseQuery = "SELECT id, max_activations, revoked FROM license_keys WHERE license = $1;"

const findLicenseUsers = "SELECT COUNT(*) FROM accounts WHERE license_id = $1;"

const createAccountQuery = "INSERT INTO accounts(license_id, code, token, first_name, last_name) VALUES ($1, $2, $3, $4, $5) RETURNING id;"

const discoverAccountQuery = "SELECT id, first_name, last_name FROM accounts WHERE code = $1;"

const authenticateQuery = "SELECT COUNT(*) FROM accounts WHERE id = $1 AND token = $2;"
