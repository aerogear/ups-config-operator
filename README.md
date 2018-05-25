# Operator to sync UPS Variants with Mobile Clients

[![Go Report Card](https://goreportcard.com/badge/github.com/aerogear/ups-config-operator)](https://goreportcard.com/report/github.com/aerogear/ups-config-operator)
[![CircleCI](https://circleci.com/gh/aerogear/ups-config-operator.svg?style=svg)](https://circleci.com/gh/aerogear/ups-config-operator)

This is an Operator that keeps variants in the [Unifiedpush Server](https://github.com/aerogear/aerogear-unifiedpush-server) in sync with your mobile project on Openshift.
It allows you to use [bindings](https://github.com/openservicebrokerapi/servicebroker/blob/master/spec.md#binding) to create variants for your mobile clients, monitors those variants and keeps them in sync should they be deleted on either UPS or Openshift.
When creating new variants this Operator will also annotate your mobile clients with all the information required for the mobile UI.

## Permissions

Currently this needs to use a service account with admin permissions. Use:

```sh
$ kubectl create clusterrolebinding <your namespace>-admin-binding --clusterrole=admin --serviceaccount=<your namespace>:default
```

# Development:

* Install Mockery on your machine: <https://github.com/vektra/mockery>       
* Run `make setup`
* Run tests: `make test`

## Usage

Make sure that you are logged in with `oc` and use the right namespace.

```
$ make build_linux
$ docker build -t docker.io/aerogear/ups-config-operator:latest -f Dockerfile .
$ oc create -f template.json
```
