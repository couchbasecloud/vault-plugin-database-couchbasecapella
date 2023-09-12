# vault-plugin-database-couchbasecapella

[![CircleCI](https://circleci.com/gh/hashicorp/vault-plugin-database-couchbase.svg?style=svg)](https://circleci.com/gh/hashicorp/vault-plugin-database-couchbasecapella)

A [Vault](https://www.vaultproject.io) plugin for Couchbase Capella

This project uses the database plugin interface introduced in Vault version 0.7.1.

The plugin supports the generation of static and dynamic user roles and root credential rotation.


## Build

To build this package for any platform you will need to clone this repository and cd into the repo directory and `go build -o couchbasecapella-database-plugin ./cmd/couchbasecapella-database-plugin/`.
To run some of the tests, create a provisioned Capella cluster instance and run it locally similar to the below example.

Set env variables and run the below: export ORG_ID=<>; export PROJECT_ID=<>; export CLUSTER_ID=<>; export ADMIN_USER_ACCESS_KEY=<>; export ADMIN_USER_SECRET_KEY=<>
```bash
make test
```
(or using the flags)
```bash
go test -v \
-apiUrl='https://cloudapi.dev.nonprod-project-avengers.com/v4' \
-orgId='6af08c0a-8cab-4c1c-b257-b521575c16d0' \
-projectId='d352361d-8de1-445b-9969-873b6decb63a' \
-clusterId='92236592-9f74-475e-afeb-0609b743c41b' \
-adminUserAccessKey='NbMSRuuUlVNuvFCLNDVc9Hk9xvoqMbKP' \
-adminUserSecretKey='eNJMuUwHP95vxSftiBsjO2WPS1znWcIQlng64PKIkUjCf5#yBVWDS8tFtFnGt7es'
```


## Installation

The Vault plugin system is documented on the [Vault documentation site](https://www.vaultproject.io/docs/internals/plugins.html).

You will need to define a plugin directory using the `plugin_directory` configuration directive, then place the
`vault-plugin-database-couchbasecapella` executable generated above, into the directory.

**Please note:** Versions v0.2.0 onwards of this plugin are incompatible with Vault versions before 1.6.0 due to an update of the database plugin interface.

Sample commands for registering and starting to use the plugin:

```bash
SHA256=$(shasum -a 256 plugins/couchbasecapella-database-plugin | cut -d' ' -f1)

vault secrets enable database

vault write sys/plugins/catalog/database/couchbasecapella-database-plugin sha256=$SHA256 \
        command=couchbasecapella-database-plugin
```

At this stage you are now ready to initialize the plugin to connect to couchbase capella cluster using unencrypted or encrypted communications.

Prior to initializing the plugin, ensure that you have created a couchbase capella provisioned cluster along with V4 API keys. Vault will use the user specified settings here to create/update/revoke database credentials. That user must have the appropriate permissions to perform actions upon other database users.

### Plugin initialization

#### Set Vault Address to the local or hosted server

```bash
export VAULT_ADDR=http://127.0.0.1:8200
```

#### Set the Capella required password policy

```bash

cat >password_policy.hcl << EOF
length=64

rule "charset" {
  charset = "abcdefghijklmnopqrstuvwxyz"
  min-chars = 1
}

rule "charset" {
  charset = "ABCDEFGHIJKLMNOPQRSTUVWXYZ"
  min-chars = 1
}

rule "charset" {
  charset = "0123456789"
  min-chars = 1
}

rule "charset" {
  charset = "#@%!"
  min-chars = 1
}
EOF

vault write sys/policies/password/couchbasecapella \
   policy=@password_policy.hcl
```  

<code>Success! Data written to: sys/policies/password/couchbasecapella</code>

#### Initialize the database plugin 

<code># Usage
vault write database/config/couchbasecapella-database \
    plugin_name="couchbasecapella-database-plugin" \
    cloud_api_base_url="https://cloudapi.dev.nonprod-project-avengers.com/v4" \
    organization_id="<org_uuid>" \
    project_id="<proj_uuid>" \
    cluster_id="<cluster_uuid>" \
    username="<v4-access-api-key>" \
    password='<v4-secret-api-key>' \
    password_policy="couchbasecapella" \
    allowed_roles="*"
</code>

```bash
vault write database/config/couchbasecapella-database \
    plugin_name="couchbasecapella-database-plugin" \
    cloud_api_base_url="https://cloudapi.dev.nonprod-project-avengers.com/v4" \
    organization_id="6af08c0a-8cab-4c1c-b257-b521575c16d0" \
    project_id="d352361d-8de1-445b-9969-873b6decb63a" \
    cluster_id="47820643-6e1f-4fea-b63c-625d1e10b536" \
    username="haI3UAw1VEOGvxjBFWDEBhnpB0nF74qf" \
    password='EUlfonfGtx#s1RlqPYryIlmcBgWuTIaMgvJ6%lrQIdu5QwCDW5oJRdeVwa3qynh7' \
    password_policy="couchbasecapella" \
    allowed_roles="*"
```
<code>Success! Data written to: database/config/couchbasecapella-database</code>

You should consider rotating the root password (same as secretKey). Note that if you do, the new password(secret) will never be made available through Vault, so you should create a vault-specific database admin user for this.

```bash
vault write -force database/rotate-root/couchbasecapella-database
```
<code>Success! Data written to: database/rotate-root/couchbasecapella-database</code>

### Dynamic Role Creation

When you create roles, you need to provide a JSON string containing the access with Couchbase RBAC roles which are documented [here](http://cbc-cp-api.s3-website-us-east-1.amazonaws.com/#tag/databaseCredentials/operation/postDatabaseCredential).

NOTE: if a creation_statement is not provided readonly for all buckets(with all scopes and collections), <code>'{access: [{ privileges: [ data_reader ], resources: { buckets: [ { name :* } ] } }]}'</code>

#### dynamicrole1 with a specific bucket, scope with both data read and write.

```bash
vault write database/roles/dynamicrole1 \
db_name="couchbasecapella-database" \
creation_statements='{"access": [ { "privileges": [ "data_reader", "data_writer" ], "resources": { "buckets": [ { "name": "vault-bucket-1", "scopes": [ { "name": "vault-bucket-1-scope-1", "collections": [ "*" ] } ] } ] } } ]}' \
default_ttl="5m" \
max_ttl="1h"
```

<code>Success! Data written to: database/roles/dynamicrole1</code>

#### dynamicrole2 with a list of 3 buckets (its all scopes &collections) access previleges of both data read and write.

```bash
vault write database/roles/dynamicrole2 \
db_name="couchbasecapella-database" \
creation_statements='{"access": [ { "privileges": [ "data_reader", "data_writer" ], "resources": { "buckets": [ { "name": "db-cred-test-12Qj", "scopes": [ { "name": "*" } ] }, { "name": "db-cred-test-3zRb", "scopes": [ { "name": "*" } ] }, { "name": "db-cred-test-FcAv", "scopes": [ { "name": "*" } ] } ] } } ]}' \
default_ttl="5m" \
max_ttl="1h" 
```

<code>Success! Data written to: database/roles/dynamicrole2</code>

#### dynamicrole3 with all buckets
```bash
vault write database/roles/dynamicrole3 \
db_name="couchbasecapella-database" \
creation_statements='{"access": [ { "privileges": [ "data_reader" ], "resources": { "buckets": [ { "name": "*" } ] } } ]}' \
default_ttl="5m" \
max_ttl="1h"
```

<code>Success! Data written to: database/roles/dynamicrole3</code>


To retrieve the credentials for the dynamic accounts

```bash
vault read database/creds/dynamicrole1
```
<code>Key                Value
    ---                -----
    lease_id           database/creds/dynamicrole1/qyaXNzyh53U1w5zIlSHAR7be
    lease_duration     5m
    lease_renewable    true
    password           !maOKglPc7IIccb!CftAC7rLsXQlxUvGKgpEnzzAYpbiifcYfVpF3E8jiyNvABtN
    username           V_TOKEN_MYDYNAMICROLE3_OAKVWMKBMT1P9IGJS1TT_1692391736
</code>


```bash
vault read database/creds/dynamicrole2
```

<code>Key                Value
        ---                -----
        lease_id           database/creds/dynamicrole2/spZ3NGneJYkptKnTgru8MJlb
        lease_duration     5m
        lease_renewable    true
        password           AVPuT#oF0cGzPIfBnaqMGsHjm9vorXwaOUw8ezP7b1k@mbTwBcRL72c@7@NDDyr0
        username           V_TOKEN_MYDYNAMICROLE3_8O0M5CKNWNSF2EA6VYLW_1692391739
</code>

```bash
vault read database/creds/dynamicrole3
```

<code>Key                Value
        ---                -----
        lease_id           database/creds/dynamicrole3/5ok4TllgYHg6QH4vsawqt2mY
        lease_duration     5m
        lease_renewable    true
        password           VVE2T4AkW%LsUgBGsY5c8opt09gB2vWhefUyJ@%qPxy%saO56d4ZPntiHkQDmiUf
        username           V_TOKEN_MYDYNAMICROLE3_ZOFAJPGLNZNQMSZCBUFK_1692391706
</code>

### Static Role Creation

In order to use static roles, the database credential user must already exist in the Couchbase Capella security settings. The example below assumes that there is an existing user with the name "vault-edu". 


```bash

# Usage: 
vault write database/static-roles/<role-name> db_name=couchbasecapella-database username="<db-cred-user-name>" rotation_period=<secs> 
```

<code>Success! Data written to: database/static-roles/<role-name></code>

```bash
# Example:
vault write database/static-roles/static-account db_name=couchbasecapella-database \
        username="vault-edu" rotation_period="5m"
```

<code>Success! Data written to: database/static-roles/static-account</code>

To retrieve the credentials for the vault-edu user

```bash
vault read database/static-creds/static-account
```

<code>Key                    Value
          ---                    -----
          last_vault_rotation    2023-08-04T19:28:21.229382-07:00
          password               wFgNaxdH2wGw2i9-B0Qj
          rotation_period        5m
          ttl                    4m59s
          username               vault-edu
</code>

## Developing

You can run `make dev` in the root of the repo to start up a development vault server and automatically register a local build of the plugin. You will need to have a built `vault` binary available in your `$PATH` to do so.
