include boilerplate/generated-includes.mk
GOLANGCI_OPTIONAL_CONFIG=./lint.conf

.PHONY: boilerplate-update
boilerplate-update:
	@boilerplate/update
