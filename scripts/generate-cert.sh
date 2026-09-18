#!/bin/sh
set -eu

if ! command -v openssl >/dev/null 2>&1; then
    echo "OpenSSL is required to generate local certificates." >&2
    exit 1
fi

script_dir=$(CDPATH= cd -- "$(dirname -- "$0")" && pwd)
project_dir=$(dirname "$script_dir")
certificate_dir="$project_dir/certs"
certificate_file="$certificate_dir/localhost.crt"
key_file="$certificate_dir/localhost.key"

if [ -e "$certificate_file" ] || [ -e "$key_file" ]; then
    echo "Certificate or private key already exists under certs/." >&2
    echo "Remove them explicitly before generating a new pair." >&2
    exit 1
fi

mkdir -p "$certificate_dir"

# Ensure newly created private files start with restrictive permissions.
umask 077

openssl req \
    -x509 \
    -newkey rsa:3072 \
    -sha256 \
    -days 365 \
    -nodes \
    -keyout "$key_file" \
    -out "$certificate_file" \
    -subj "/CN=localhost" \
    -addext "subjectAltName=DNS:localhost,IP:127.0.0.1" \
    -addext "keyUsage=critical,digitalSignature,keyEncipherment" \
    -addext "extendedKeyUsage=serverAuth"

chmod 600 "$key_file"
chmod 644 "$certificate_file"

echo "Created:"
echo "  certs/localhost.crt"
echo "  certs/localhost.key"