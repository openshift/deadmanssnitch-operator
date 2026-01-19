FROM quay.io/redhat-services-prod/openshift/boilerplate:image-v8.3.1 AS builder
COPY . /go/src/github.com/openshift/deadmanssnitch-operator
WORKDIR /go/src/github.com/openshift/deadmanssnitch-operator
ENV GOFLAGS=""
RUN make go-build

FROM registry.access.redhat.com/ubi9/ubi-minimal:9.7-1768783948
ENV OPERATOR_BIN=deadmanssnitch-operator

WORKDIR /root/
COPY --from=builder /go/src/github.com/openshift/deadmanssnitch-operator/build/_output/bin/${OPERATOR_BIN} /usr/local/bin/${OPERATOR_BIN}
LABEL io.openshift.managed.name="deadmanssnitch-operator" \
      io.openshift.managed.description="Operator to manage Dead Man Snitches"
