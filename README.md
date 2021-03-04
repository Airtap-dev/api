# Airtap Server
This project manages the SDP relay and the minimal business logic needed to get to Airtap 1.0.

It compiles to multiple binaries and runs on Heroku.

The `issuer` executable issues new license codes on demand (defaults to 10 accounts per key). The `api` executable serves the HTTP API and the SDP relay over WebSockets.

## Running
Once you have your local Heroku environment set up, make sure you are working with the **staging** environment.
```
git pull
git checkout staging
heroku repo:reset -a airtap-api-staging
heroku git:remote -a staging
```

Then you can build:
```
make build
./bin/issuer
./bin/api
```

And you can run (Go is a compiled language, make sure to rebuild before each run):
`heroku local` (runs the issuer) or `heroku local web` (runs the web server).

When running locally with `heroku local`, you can set environment variables in the `.env` file. It's already in `.gitignore`.

To issue a new license on staging, run:
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