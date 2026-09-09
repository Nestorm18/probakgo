package selfupdate

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/sigstore/sigstore-go/pkg/fulcio/certificate"
)

func TestReleaseIdentityRejectsWrongSource(t *testing.T) {
	const repo, tag, commit = "Nestorm18/probakgo", "v0.0.242", "cacb6de8d716742e7bbaa3cbd09803d695d93b48"
	policy, err := releaseIdentity(repo, tag, commit)
	if err != nil {
		t.Fatal(err)
	}
	cert := certificate.Summary{
		SubjectAlternativeName: "https://github.com/" + repo + "/.github/workflows/release.yml@refs/heads/master",
		Extensions:             certificate.Extensions{Issuer: "https://token.actions.githubusercontent.com", SourceRepositoryURI: "https://github.com/" + repo, SourceRepositoryDigest: commit, RunnerEnvironment: "github-hosted"},
	}
	if err := policy.Verify(cert); err != nil {
		t.Fatalf("legitimate release identity rejected: %v", err)
	}
	for _, change := range []func(*certificate.Summary){
		func(c *certificate.Summary) { c.Issuer = "https://attacker.example" },
		func(c *certificate.Summary) { c.SourceRepositoryURI = "https://github.com/attacker/probakgo" },
		func(c *certificate.Summary) { c.SourceRepositoryDigest = strings.Repeat("0", 40) },
		func(c *certificate.Summary) { c.RunnerEnvironment = "self-hosted" },
		func(c *certificate.Summary) {
			c.SubjectAlternativeName = strings.Replace(c.SubjectAlternativeName, "release.yml", "ci.yml", 1)
		},
		func(c *certificate.Summary) {
			c.SubjectAlternativeName = strings.Replace(c.SubjectAlternativeName, "heads/master", "heads/untrusted", 1)
		},
		func(c *certificate.Summary) {
			c.SubjectAlternativeName = strings.Replace(c.SubjectAlternativeName, "heads/master", "tags/v0.0.241", 1)
		},
	} {
		other := cert
		change(&other)
		if err := policy.Verify(other); err == nil {
			t.Fatalf("untrusted identity accepted: %+v", other)
		}
	}
	cert.SubjectAlternativeName = strings.Replace(cert.SubjectAlternativeName, "heads/master", "tags/"+tag, 1)
	if err := policy.Verify(cert); err != nil {
		t.Fatal(err)
	}
}

func TestAttestationFailsClosedWhenAbsentOrMalformed(t *testing.T) {
	old := githubAPIBaseURL
	t.Cleanup(func() { githubAPIBaseURL = old })
	t.Setenv("GITHUB_TOKEN", "")
	for _, body := range []string{`{"attestations":[]}`, `{"attestations":[{"bundle":{}}]}`} {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Type", "application/json")
			if strings.Contains(r.URL.Path, "/commits/") {
				_ = json.NewEncoder(w).Encode(map[string]string{"sha": strings.Repeat("a", 40)})
				return
			}
			_, _ = w.Write([]byte(body))
		}))
		githubAPIBaseURL = server.URL
		err := verifyReleaseAttestation("Nestorm18/probakgo", "v0.0.242", strings.Repeat("1", 64))
		server.Close()
		if err == nil {
			t.Fatal("unattested update accepted")
		}
	}
	if _, err := attestationVerifier(context.Background(), "attacker CA"); err == nil {
		t.Fatal("unknown authority accepted")
	}
}

func TestGitHubBootstrapIsPinned(t *testing.T) {
	digest := sha256.Sum256(githubTUFRoot)
	if hex.EncodeToString(digest[:]) != "98cba97be9075bc98b2322de3de85fbd1b70ec7392991dfd2f53d215bede1a8d" {
		t.Fatal("GitHub trust bootstrap changed without review")
	}
}

// Explicit opt-in: this verifies public metadata only, without downloading or
// executing a binary. Ordinary test runs remain independent of network access.
func TestReleaseAttestationIntegration(t *testing.T) {
	if os.Getenv("PROBAKGO_TEST_ATTESTATIONS") != "1" {
		t.Skip("set PROBAKGO_TEST_ATTESTATIONS=1 for live signature verification")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()
	if _, err := attestationVerifier(ctx, "GitHub, Inc."); err != nil {
		t.Fatalf("GitHub private-repository trust bootstrap: %v", err)
	}
	const digest = "7ff52b5c52aa726f999e029c0ca3441cf26e4a3ad49fa34953c9ac2a0595afa8"
	var attestations json.RawMessage
	if err := fetchAttestationJSON(ctx, &http.Client{Timeout: 15 * time.Second}, "/repos/Nestorm18/probakgo/attestations/sha256:"+digest, &attestations); err != nil {
		t.Fatal(err)
	}
	// v0.0.242's tag differs from its signed source. Preserve the actual signed
	// source commit here, and exercise rejection of a mismatched tag separately.
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.Contains(r.URL.Path, "/commits/") {
			commit := "cacb6de8d716742e7bbaa3cbd09803d695d93b48"
			if strings.HasSuffix(r.URL.Path, "/v0.0.241") {
				commit = strings.Repeat("a", 40)
			}
			_ = json.NewEncoder(w).Encode(map[string]string{"sha": commit})
			return
		}
		_, _ = w.Write(attestations)
	}))
	defer server.Close()
	old := githubAPIBaseURL
	githubAPIBaseURL = server.URL
	defer func() { githubAPIBaseURL = old }()
	if err := verifyReleaseAttestation("Nestorm18/probakgo", "v0.0.242", digest); err != nil {
		t.Fatal(err)
	}
	if err := verifyReleaseAttestation("Nestorm18/probakgo", "v0.0.242", strings.Repeat("0", 64)); err == nil {
		t.Fatal("signature accepted for a different binary")
	}
	if err := verifyReleaseAttestation("Nestorm18/probakgo", "v0.0.241", digest); err == nil {
		t.Fatal("signature accepted for a different source commit")
	}
}
