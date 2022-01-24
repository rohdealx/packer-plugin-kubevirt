#!/usr/bin/env bash

set -e

KIND_CLUSTER_NAME="${2:-kubevirt}"

if [ "${1}" = "setup" ]
then
    KUBECONFIG="${PWD}/kubeconfig"
    KUBEVIRT_VERSION=$(curl -s 'https://api.github.com/repos/kubevirt/kubevirt/releases' | grep tag_name | grep -v -- '-rc' | sort -r | head -1 | awk -F': ' '{print $2}' | sed 's/,//' | xargs)
    CDI_VERSION=$(curl -s 'https://github.com/kubevirt/containerized-data-importer/releases/latest' | grep -o 'v[0-9]\.[0-9]*\.[0-9]*')
    kind create cluster --name "${KIND_CLUSTER_NAME}" --wait 30m
    kubectl create -f "https://github.com/kubevirt/kubevirt/releases/download/${KUBEVIRT_VERSION}/kubevirt-operator.yaml"
    kubectl create -f "https://github.com/kubevirt/kubevirt/releases/download/${KUBEVIRT_VERSION}/kubevirt-cr.yaml"
    kubectl create -f "https://github.com/kubevirt/containerized-data-importer/releases/download/${CDI_VERSION}/cdi-operator.yaml"
    kubectl create -f "https://github.com/kubevirt/containerized-data-importer/releases/download/${CDI_VERSION}/cdi-cr.yaml"
    kubectl -n kubevirt wait --for=condition=Available --timeout=30m kubevirt/kubevirt
    kubectl -n cdi wait --for=condition=Available --timeout=30m cdi/cdi
elif [ "${1}" = "teardown" ]
then
    kind delete cluster --name "${KIND_CLUSTER_NAME}"
else
    echo 'Available commands are: setup, teardown'
fi
