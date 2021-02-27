package main

const issueQuery = "INSERT INTO license_keys(max_activations) VALUES ($1) RETURNING license, max_activations, revoked;"
