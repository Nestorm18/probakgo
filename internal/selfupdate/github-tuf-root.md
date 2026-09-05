GitHub TUF bootstrap root, version 3

Source: https://raw.githubusercontent.com/cli/cli/31e17047d1b82244d11a780e250440a6f855a205/pkg/cmd/attestation/verification/embed/tuf-repo.github.com/root.json

Independently compared byte for byte with https://tuf-repo.github.com/3.root.json.

SHA-256: `98cba97be9075bc98b2322de3de85fbd1b70ec7392991dfd2f53d215bede1a8d`.

This public key metadata bootstraps TUF's signed root rotation. The verifier
requires fresh, authenticated metadata and does not disable expiry checks.
Public repositories use Sigstore's bootstrap embedded in sigstore-go instead.
