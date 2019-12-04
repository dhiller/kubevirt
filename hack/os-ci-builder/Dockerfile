FROM kubevirt/builder@sha256:0cc33e4e2a04aa4ec44a9c38c37ff926a8b2ca67181200e31c48298bff79c4b2

RUN curl -L -O -o openshift-origin-client-tools-v3.11.0-0cbc58b-linux-64bit.tar.gz https://github.com/openshift/origin/releases/download/v3.11.0/openshift-origin-client-tools-v3.11.0-0cbc58b-linux-64bit.tar.gz \
    && tar zxvf openshift-origin-client-tools-v3.11.0-0cbc58b-linux-64bit.tar.gz \
    && cd openshift-origin-client-tools-v3.11.0-0cbc58b-linux-64bit \
    && install kubectl /usr/local/bin \
    && install oc /usr/local/bin

WORKDIR /go/src/kubevirt.io/kubevirt