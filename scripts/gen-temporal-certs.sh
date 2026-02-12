#!/usr/bin/env bash
#
# Generate self-signed CA + server + client certificates for Temporal mTLS.
# Output: certs/temporal/{ca,server,client}.{pem,key}
#
set -euo pipefail

CERT_DIR="certs/temporal"
DAYS_CA=3650
DAYS_CERT=365

mkdir -p "$CERT_DIR"

echo "==> Generating CA key + certificate"
openssl genrsa -out "$CERT_DIR/ca.key" 4096 2>/dev/null
openssl req -new -x509 -days "$DAYS_CA" \
    -key "$CERT_DIR/ca.key" \
    -out "$CERT_DIR/ca.pem" \
    -subj "/CN=Hosting Temporal CA"

echo "==> Generating server certificate"
openssl genrsa -out "$CERT_DIR/server.key" 2048 2>/dev/null
openssl req -new \
    -key "$CERT_DIR/server.key" \
    -out "$CERT_DIR/server.csr" \
    -subj "/CN=temporal"

cat > "$CERT_DIR/server-ext.cnf" <<EOF
subjectAltName = DNS:temporal,DNS:localhost,IP:127.0.0.1
EOF

openssl x509 -req -days "$DAYS_CERT" \
    -in "$CERT_DIR/server.csr" \
    -CA "$CERT_DIR/ca.pem" \
    -CAkey "$CERT_DIR/ca.key" \
    -CAcreateserial \
    -extfile "$CERT_DIR/server-ext.cnf" \
    -out "$CERT_DIR/server.pem" 2>/dev/null

echo "==> Generating client certificate"
openssl genrsa -out "$CERT_DIR/client.key" 2048 2>/dev/null
openssl req -new \
    -key "$CERT_DIR/client.key" \
    -out "$CERT_DIR/client.csr" \
    -subj "/CN=hosting-client"

openssl x509 -req -days "$DAYS_CERT" \
    -in "$CERT_DIR/client.csr" \
    -CA "$CERT_DIR/ca.pem" \
    -CAkey "$CERT_DIR/ca.key" \
    -CAcreateserial \
    -out "$CERT_DIR/client.pem" 2>/dev/null

# Clean up intermediate files
rm -f "$CERT_DIR"/*.csr "$CERT_DIR"/*.cnf "$CERT_DIR"/*.srl

echo "==> Certificates generated in $CERT_DIR/"
ls -la "$CERT_DIR/"
