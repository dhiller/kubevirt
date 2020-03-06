#!/usr/bin/env bash
set -xeuo pipefail

export DOCKER_PREFIX='dhiller'
export DOCKER_TAG="latest"
export KUBEVIRT_PROVIDER=external

echo "calling cluster-up to prepare config and check whether cluster is reachable"
# TODO: remove patching of external provider, after kubevirtci#199 has been merged and kubevirt updates
(
  cd cluster-up/cluster/external \
  && curl -L -O https://raw.githubusercontent.com/dhiller/kubevirtci/fix-external-provider/cluster-up/cluster/external/provider.sh
)
bash -x ./cluster-up/up.sh

echo "deploying"
bash -x ./hack/cluster-deploy.sh

echo "testing"
mkdir -p "$ARTIFACT_DIR"
TESTS_TO_FOCUS=$(grep -E -o '\[crit\:high\]' tests/*_test.go | sort | uniq | sed -E 's/tests\/([a-z_]+)\.go\:.*/\1/' | tr '\n' '|' | sed 's/|$//')
FUNC_TEST_ARGS='--ginkgo.noColor --ginkgo.focus='"$TESTS_TO_FOCUS"' --ginkgo.regexScansFilePath=true --junit-output='"$ARTIFACT_DIR"'/junit.functest.xml' \
  bash -x ./hack/functests.sh
