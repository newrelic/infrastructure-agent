#!/bin/bash
set -e

# Configuration
NAME_REAL="${GPG_KEY_NAME:-New Relic Infrastructure Agent}"
NAME_EMAIL="${GPG_KEY_EMAIL:-infrastructure-eng@newrelic.com}"
PASSPHRASE="${OHAI_GPG_PASSPHRASE}"

echo "Generating GPG key with SHA256..."

# Create GPG batch configuration
cat > gpg-batch-config.txt <<EOF
Key-Type: RSA
Key-Length: 4096
Key-Usage: sign
Subkey-Type: RSA
Subkey-Length: 4096
Subkey-Usage: encrypt
Name-Real: ${NAME_REAL}
Name-Email: ${NAME_EMAIL}
Expire-Date: 0
Passphrase: ${PASSPHRASE}
Preferences: SHA256 SHA384 SHA512 AES256 AES192 AES
%commit
EOF

# Generate the key
gpg --batch --generate-key gpg-batch-config.txt

# Get the key ID
KEY_ID=$(gpg --list-keys --with-colons "${NAME_EMAIL}" | awk -F: '/^pub:/ {print $5}' | head -n 1)

echo "Key ID: ${KEY_ID}"

# Export keys with passphrase
gpg --batch --pinentry-mode loopback --passphrase "${PASSPHRASE}" --armor --export-secret-keys "${KEY_ID}" > gpg-private-key.asc
gpg --armor --export "${KEY_ID}" > gpg-public-key.asc

# Clean up
rm -f gpg-batch-config.txt

echo "✓ GPG keys generated successfully with SHA256"
echo "  Email: ${NAME_EMAIL}"
echo "  Expiration: Never"
echo "  Private key: gpg-private-key.asc"
echo "  Public key: gpg-public-key.asc"
