
.PHONY: fmt-check-files
fmt-check-files:
	@go tool gofumpt -l .

.PHONY: fmt-check-diff
fmt-check-diff:
	@go tool gofumpt -d .

.PHONY: fmt
fmt:
	@go tool gofumpt -w .

.PHONY: test
test:
	@go test ./...

.PHONY: lint
lint:
	@go vet ./...
	@go tool staticcheck

.PHONY: check
check: fmt-check-diff lint test


.PHONY: list
list: ## List all make targets
	@${MAKE} -pRrn : -f $(MAKEFILE_LIST) 2>/dev/null | awk -v RS= -F: '/^# File/,/^# Finished Make data base/ {if ($$1 !~ "^[#.]") {print $$1}}' | grep -v -e '^[^[:alnum:]]' -e '^$@$$' | sort

.PHONY: help
.DEFAULT_GOAL := help
help:
	@grep -h -E '^[a-zA-Z_-]+:.*?## .*$$' $(MAKEFILE_LIST) | awk 'BEGIN {FS = ":.*?## "}; {printf "\033[36m%-30s\033[0m %s\n", $$1, $$2}'