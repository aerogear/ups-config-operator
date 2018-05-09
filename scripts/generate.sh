#!/usr/bin/env bash

k8=$GOPATH/src/kubernetes
code_gen=${k8}/code-generator


if [ ! -d ${code_gen} ]; then
   mkdir -p ${k8} && cd $k8 && git clone git@github.com:kubernetes/code-generator.git
fi

cd ${code_gen} && git checkout release-1.8

cd ${code_gen}
./generate-internal-groups.sh client github.com/aerogear/ups-config-operator/pkg/client/mobile github.com/aerogear/ups-config-operator/pkg/apis github.com/aerogear/ups-config-operator/pkg/apis  "mobile:v1alpha1"
./generate-internal-groups.sh client github.com/aerogear/ups-config-operator/pkg/client/servicecatalog github.com/aerogear/ups-config-operator/pkg/apis github.com/aerogear/ups-config-operator/pkg/apis "servicecatalog:v1beta1"