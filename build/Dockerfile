FROM quay.io/app-sre/boilerplate:image-v3.0.3 AS builder
COPY . /go/src/github.com/openshift/deadmanssnitch-operator
WORKDIR /go/src/github.com/openshift/deadmanssnitch-operator
ENV GOFLAGS=""
RUN make go-build

FROM registry.access.redhat.com/ubi8/ubi-minimal:8.8-860
ENV OPERATOR_BIN=deadmanssnitch-operator

WORKDIR /root/
COPY --from=builder /go/src/github.com/openshift/deadmanssnitch-operator/build/_output/bin/${OPERATOR_BIN} /usr/local/bin/${OPERATOR_BIN}
LABEL io.openshift.managed.name="deadmanssnitch-operator" \
      io.openshift.managed.description="Operator to manage Dead Man Snitches"
