.PHONY: run cert verify-cert

cert:
	@./scripts/generate-cert.sh

verify-cert:
	@openssl x509 \
		-in certs/localhost.crt \
		-noout \
		-subject \
		-issuer \
		-dates \
		-ext subjectAltName

run:
	@set -a; . ./.env; set +a; go run ./cmd/forum