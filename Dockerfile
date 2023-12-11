# Stage 1: Build the Vault plugin
FROM golang:1.21 as builder

ARG PLUGIN_NAME=couchbasecapella-database-plugin
ARG PLUGIN_DIR=/vault/plugins/

# Install Git
RUN apt-get update && \
    apt-get install -y git

# Set up the Go workspace
WORKDIR /go/src/github.com/couchbaselabs/${PLUGIN_NAME}

# Clone the plugin repository
ADD . .

# Build the plugin
RUN CGO_ENABLED=0 GOOS=linux go build -o ${PLUGIN_DIR}/${PLUGIN_NAME} "./cmd/couchbasecapella-database-plugin"
RUN shasum -a 256 "${PLUGIN_DIR}/${PLUGIN_NAME}" | cut -d " " -f1 > ${PLUGIN_DIR}/${PLUGIN_NAME}.sha256

# Stage 2: Add the plugin to the Vault image
FROM hashicorp/vault:1.15

ARG PLUGIN_NAME=couchbasecapella-database-plugin
ARG PLUGIN_DIR=/vault/plugins

ENV PLUGIN_NAME=$PLUGIN_NAME
ENV PLUGIN_DIR=$PLUGIN_DIR

# Set environment variables for Vault
ENV VAULT_ADDR=http://127.0.0.1:8200
ENV VAULT_API_ADDR=http://127.0.0.1:8200

# Copy the plugin binary from the builder stage
COPY --from=builder ${PLUGIN_DIR}/${PLUGIN_NAME} ${PLUGIN_DIR}/${PLUGIN_NAME}
COPY --from=builder ${PLUGIN_DIR}/${PLUGIN_NAME}.sha256 /vault/${PLUGIN_NAME}.sha256

COPY <<EOF /vault/password_policy.hcl
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

# Update Vault's configuration to load the plugin
RUN echo '{"plugin_directory": "/vault/plugins", "storage": {"file": {"path": "/vault/file"}}, "default_lease_ttl": "168h", "max_lease_ttl": "720h", "ui": true}' > /vault/config/config.json

# Expose Vault ports
EXPOSE 8200