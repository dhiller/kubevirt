#!/bin/bash

set -euo pipefail

export GIMME_GO_VERSION=1.12.8
export GOPATH="/go"
export GOBIN="/usr/bin"
source /etc/profile.d/gimme.sh

export DOCKER_PREFIX='docker.io/dhiller/kubevirt'
export DOCKER_TAG="$( echo "${JOB_NAME}-${BUILD_ID}" | md5sum -t | cut -c 1-8 )"
export KUBEVIRT_PROVIDER=external

echo "building and pushing images"
bash -x ./hack/bazel-push-images.sh

echo "creating manifests"
bash -x ./hack/build-manifests.sh

echo "deploying"
bash -x ./hack/cluster-deploy.sh

echo "testing"
bash -x ./hack/build-func-tests.sh"
bash -x ./hack/functests.sh
