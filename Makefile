include boilerplate/generated-includes.mk

.PHONY: boilerplate-update
boilerplate-update:
	@boilerplate/update

# ==> HACK ==>
# We have to tweak the YAML to run this on 3.11.
# We'll add a target to do that, then override the `generate` target to
# call it.
.PHONY: crd-fixup
crd-fixup:
	yq d -i deploy/crds/deadmanssnitch.managed.openshift.io_deadmanssnitchintegrations_crd.yaml spec.validation.openAPIV3Schema.type

# NOTE: go-generate is a no-op at the moment.
generate: op-generate crd-fixup go-generate
# <== HACK <==
