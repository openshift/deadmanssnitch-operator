FROM registry.svc.ci.openshift.org/openshift/release:golang-1.10 AS builder
COPY . /go/src/github.com/openshift/deadmanssnitch-operator
WORKDIR /go/src/github.com/openshift/deadmanssnitch-operator
RUN make gobuild

FROM registry.svc.ci.openshift.org/openshift/origin-v4.0:base
ENV OPERATOR_BIN=deadmanssnitch-operator

WORKDIR /root/
COPY --from=builder /go/src/github.com/openshift/deadmanssnitch-operator/build/_output/bin/${OPERATOR_BIN} /usr/local/bin/${OPERATOR_BIN}
LABEL io.openshift.managed.name="deadmanssnitch-operator" \
      io.openshift.managed.description="Operator to manage Dead Man Snitches"
