test:
	go test ./...
	cd web && npm test
	cd npm && npm test

vet:
	go vet ./...

race:
	go test -race ./internal/cli ./internal/table ./internal/report ./internal/metric ./internal/fuzzy ./internal/vendor ./internal/httpapi ./internal/adapter/trae

cli-fixture:
	bash scripts/verify-cli.sh

build-all:
	bash scripts/build-all.sh

install-script:
	bash -n scripts/install.sh

govulncheck:
	bash scripts/govulncheck.sh

fmt-check:
	test -z "$$(gofmt -l .)"

ci: fmt-check vet test race cli-fixture

.PHONY: test vet race cli-fixture build-all install-script govulncheck fmt-check ci
