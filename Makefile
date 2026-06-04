TEMPLATE ?= admin
FBA_GO_ROOT ?= ../fba-go

.PHONY: verify
verify:
	FBA_GO_ROOT=$(FBA_GO_ROOT) scripts/verify-template.sh $(TEMPLATE)

.PHONY: verify-admin
verify-admin:
	FBA_GO_ROOT=$(FBA_GO_ROOT) scripts/verify-template.sh admin

