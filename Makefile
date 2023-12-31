TOOL?=vault-plugin-database-couchbasecapella
TEST?=$$(go list ./... | grep -v /vendor/ | grep -v teamcity)
VETARGS?=-asmdecl -atomic -bool -buildtags -copylocks -methods -nilfunc -printf -rangeloops -shift -structtags -unsafeptr
BUILD_TAGS?=${TOOL}
GOFMT_FILES?=$$(find . -name '*.go' | grep -v vendor)
TEST_ARGS?=-orgId='$(ORG_ID)' -projectId='$(PROJECT_ID)' -clusterId='$(CLUSTER_ID)' -adminUserAccessKey='$(ADMIN_USER_ACCESS_KEY)' -adminUserSecretKey='$(ADMIN_USER_SECRET_KEY)'
GO_TEST_CMD?=go test -v ${TEST_ARGS}

# bin generates the releaseable binaries for this plugin
bin: fmtcheck
	@CGO_ENABLED=0 BUILD_TAGS='$(BUILD_TAGS)' sh -c "'$(CURDIR)/scripts/build.sh'"

default: dev

# dev starts up `vault` from your $PATH, then builds the couchbasecapella
# plugin, registers it with vault and enables it.
# A ./tmp dir is created for configs and binaries, and cleaned up on exit.
dev: fmtcheck
	@CGO_ENABLED=0 BUILD_TAGS='$(BUILD_TAGS)' VAULT_DEV_BUILD=1 sh -c "'$(CURDIR)/scripts/build.sh'"

# test runs the unit tests and vets the code
test: fmtcheck
	CGO_ENABLED=0 VAULT_TOKEN= ${GO_TEST_CMD} -tags='$(BUILD_TAGS)' $(TEST) $(TESTARGS) -count=1 -timeout=5m -parallel=4

testacc: fmtcheck
	CGO_ENABLED=0 VAULT_TOKEN= VAULT_ACC=1 ${GO_TEST_CMD} -tags='$(BUILD_TAGS)' $(TEST) $(TESTARGS) -count=1 -timeout=20m

testcompile: fmtcheck
	@for pkg in $(TEST) ; do \
		go test -v -c -tags='$(BUILD_TAGS)' $$pkg ; \
	done

fmtcheck:
	@sh -c "'$(CURDIR)/scripts/gofmtcheck.sh'"

fmt:
	gofmt -w $(GOFMT_FILES)

.PHONY: bin default dev test testcompile fmtcheck fmt