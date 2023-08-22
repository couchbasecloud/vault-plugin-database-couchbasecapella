module github.com/hashicorp/vault-plugin-database-couchbasecapella

go 1.14

require (
	github.com/cenkalti/backoff v2.2.1+incompatible
	github.com/couchbase/gocb/v2 v2.3.3
	github.com/hashicorp/errwrap v1.1.0
	github.com/hashicorp/go-hclog v1.4.0
	github.com/hashicorp/go-secure-stdlib/strutil v0.1.2
	github.com/hashicorp/go-version v1.6.0
	github.com/hashicorp/vault/sdk v0.9.2
	github.com/lib/pq v1.8.0 // indirect
	github.com/mitchellh/mapstructure v1.5.0
	github.com/ory/dockertest/v3 v3.8.0
	github.com/stretchr/testify v1.8.2
)
