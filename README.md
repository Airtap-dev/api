# Airtap Server
This project manages the SDP relay and the minimal business logic needed to get to Airtap 1.0.

It compiles to multiple binaries and runs on Heroku.

The `issuer` executable issues new license codes on demand (defaults to 10 accounts per key). The `api` executable serves the HTTP API and the SDP relay over WebSockets.

## Running
Once you have your local Heroku environment set up, run:
```
make build
./bin/issuer
./bin/api
```

To issue a new license, run:
```
heroku run issuer -- -max=<number of activations>
```

The `issuer` Dyno is scaled to **0** so that it doesn't run on each deploy.
The `web` (aka `api`) Dyno is scaled to **1**.


## Deploying
**The app deploys automatically on each merge to the main branch.**

Manual deployment:
```
git push heroku main
heroku run issuer
```