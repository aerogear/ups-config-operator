# Sidecar service to sync UPS Variants with Mobile Clients

[![Go Report Card](https://goreportcard.com/badge/github.com/aerogear/ups-sidecar)](https://goreportcard.com/report/github.com/aerogear/ups-sidecar)

*Note* Just a POC at the moment

## Permissions

Currently this needs to use a service account with admin permissions. Use:

```sh
$ kubectl create clusterrolebinding <your namespace>-admin-binding --clusterrole=admin --serviceaccount=<your namespace>:default
```

## Usage

Make sure that you are logged in with `oc` and use the right namespace.

```
$ make build_linux
$ docker build -t docker.io/aerogear/ups-sidecar:latest -f Dockerfile .
$ oc create -f template.json
```
