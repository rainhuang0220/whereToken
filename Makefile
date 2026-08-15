test:
	go test ./...
	cd web && npm test
	cd npm && npm test

cli-fixture:
	bash scripts/verify-cli.sh

build-all:
	bash scripts/build-all.sh

install-script:
	bash -n scripts/install.sh

.PHONY: test cli-fixture build-all install-script
