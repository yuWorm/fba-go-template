TEMPLATE ?= admin
VERIFY_ENV := $(if $(FBA_GO_ROOT),FBA_GO_ROOT=$(FBA_GO_ROOT),)

.PHONY: verify
verify:
	$(VERIFY_ENV) scripts/verify-template.sh $(TEMPLATE)

.PHONY: verify-admin
verify-admin:
	$(VERIFY_ENV) scripts/verify-template.sh admin
