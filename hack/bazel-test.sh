set -e

source hack/common.sh
source hack/config.sh

bazel test \
    --config=${ARCHITECTURE} \
    --stamp \
    --features race \
    --test_output=errors -- //staging/src/kubevirt.io/client-go/... //pkg/... //cmd/...
