# Airtap Server
This project manages the SDP relay and the minimal business logic needed to get to Airtap 1.0.

It compiles to multiple binaries and runs on Heroku.

The `issuer` executable issues new license codes on demand (defaults to 10 accounts per key). The `server` executable serves the HTTP API and the SDP relay over WebSockets.

## Running
Once you have your local Heroku environment set up, run:
```
make build
./bin/issuer
./bin/server
```

## Deploying
**The app deploys automatically on each merge to the main branch.**

Manual deployment:
```
git push heroku main
heroku run issuer
```