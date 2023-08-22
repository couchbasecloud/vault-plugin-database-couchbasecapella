#!/bin/bash -x
MNT_PATH="couchbasecapella"
PLUGIN_NAME="vault-plugin-database-couchbasecapella"
PLUGIN_CATALOG_NAME="couchbasecapella-database-plugin"
SCRATCH=./tmp

go build -o "$SCRATCH/plugins/$PLUGIN_NAME" "./cmd/couchbasecapella-database-plugin"
SHASUM=$(shasum -a 256 "$SCRATCH/plugins/$PLUGIN_NAME" | cut -d " " -f1)

echo "    Registering plugin"
vault write sys/plugins/catalog/$PLUGIN_CATALOG_NAME \
  sha_256="$SHASUM" \
  command="$PLUGIN_NAME"

echo "    Mounting plugin"
vault secrets enable database
