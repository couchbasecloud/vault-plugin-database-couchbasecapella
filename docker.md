# vault-plugin-database-couchbasecapella with Docker

Please note this build is not recommended for Production use. It is designed for testing using Docker.

## Build

You can build a test Vault image where this plugin will be copied to the `/vault/plugins` folder

```bash
docker build -t vault:with-cb-capella-plugin .
```

## Setup

The following command will start Vault in development mode with a root token `password`.  This can be used to test the plugin

```bash
docker run --cap-add=IPC_LOCK --name="couchbase_vault" --rm \
    -e VAULT_DEV_LISTEN_ADDRESS=0.0.0.0:8200 \
    -e VAULT_ADDR=http://0.0.0.0:8200 \
    -p 8200:8200 \
    vault:with-cb-capella-plugin \
	vault server -dev -dev-root-token-id="password" \
    -log-level=debug -config=/vault/config/config.json
```

### Enable Database secrets

```bash
docker exec -it "couchbase_vault" /bin/ash -c "vault login password && vault secrets enable database"
```
### Register the plugin

The multi-stage docker container first builds the plugin. It also generates the SHA256 and saves it the following file:
*/vault/vault-plugin-database-couchbasecapella.sha256*

```bash
docker exec -it "couchbase_vault" /bin/ash -c "SHA256=\$(cat /vault/vault-plugin-database-couchbasecapella.sha256) && vault login password && vault write sys/plugins/catalog/database/vault-plugin-database-couchbasecapella sha256=\$SHA256 command=vault-plugin-database-couchbasecapella"
```

You can check if the plugin was registered by listing the installed plugins with the following command

```bash
docker exec -it "couchbase_vault" /bin/ash -c "vault login password && vault plugin list"
```

### Upload password policy

Couchbase provides a Vault password policy file that can be used with the plugin. The policy can be found at */vault/password_policy.hcl*

```bash
docker exec -it "couchbase_vault" /bin/ash -c "vault login password && vault write sys/policies/password/couchbasecapella policy=@/vault/password_policy.hcl"
```

## Testing

### Create database config

You can use the following command to create a database config that sets up the connection to your Capella cluster.
Make sure to replace the variables.

```bash
docker exec -it "couchbase_vault" /bin/ash -c 'vault login password && vault write database/config/vault-plugin-database-couchbasecapella plugin_name="vault-plugin-database-couchbasecapella" cloud_api_base_url="https://cloudapi.cloud.couchbase.com/v4" organization_id="$your_capella_organization_id" project_id="$your_capella_project_id" cluster_id="$your_capella_cluster_id" username="$your_capella_access_key_name" password="$your_capella_access_key_secret" password_policy="couchbasecapella" allowed_roles="*"'
```
> Please note: it uses the password policy we registered before

### Rotate root credentials

The plugin supports rotating the root credentials that was used to initialize the database config

```bash
docker exec -it "couchbase_vault" /bin/ash -c "vault login password && vault write -force database/rotate-root/vault-plugin-database-couchbasecapella"
```
### Create a dynamic role

```bash
docker exec -it "couchbase_vault" /bin/ash -c 'vault login password && vault write database/roles/dynamicrole1 db_name="vault-plugin-database-couchbasecapella" creation_statements='\''{"access": [ { "privileges": [ "data_reader", "data_writer" ], "resources": { "buckets": [ { "name": "vault-bucket-1", "scopes": [ { "name": "vault-bucket-1-scope-1", "collections": [ "*" ] } ] } ] } } ]}'\'' default_ttl="5m" max_ttl="1h"'
```

> Please note: this example assumes you have a bucket called: *vault-bucket-1* and a scope called: *vault-bucket-1-scope-1*

### Create a new credential

Using the dynamic role setup in the earlier step, we can ask Vault to create a new set of database credentials

```bash
docker exec -it "couchbase_vault" /bin/ash -c 'vault login password && vault read database/creds/dynamicrole1'
```