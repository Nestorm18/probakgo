package selfupdate

import (
	"context"
	_ "embed"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"regexp"
	"time"

	"github.com/sigstore/sigstore-go/pkg/bundle"
	"github.com/sigstore/sigstore-go/pkg/root"
	"github.com/sigstore/sigstore-go/pkg/tuf"
	"github.com/sigstore/sigstore-go/pkg/verify"
	"github.com/theupdateframework/go-tuf/v2/metadata/fetcher"
)

// GitHub CLI's pinned bootstrap, also checked against the official TUF mirror.
// See github-tuf-root.md for provenance. TUF verifies signed root rotations and
// current metadata expiration; the embedded bootstrap is never trusted as a CA.
//
//go:embed github-tuf-root.json
var githubTUFRoot []byte

func verifyReleaseAttestation(repo, tag, digest string) error {
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()
	client := &http.Client{Timeout: 15 * time.Second}
	var commit struct {
		SHA string `json:"sha"`
	}
	if err := fetchAttestationJSON(ctx, client, "/repos/"+repo+"/commits/"+url.PathEscape(tag), &commit); err != nil {
		return fmt.Errorf("resolve attested release commit: %w", err)
	}
	if !regexp.MustCompile(`^[a-f0-9]{40,64}$`).MatchString(commit.SHA) {
		return fmt.Errorf("invalid release commit digest")
	}
	var response struct {
		Attestations []struct {
			Bundle json.RawMessage `json:"bundle"`
		} `json:"attestations"`
	}
	if err := fetchAttestationJSON(ctx, client, "/repos/"+repo+"/attestations/sha256:"+digest+"?per_page=10", &response); err != nil {
		return fmt.Errorf("fetch release attestation: %w", err)
	}
	identity, err := releaseIdentity(repo, tag, commit.SHA)
	if err != nil {
		return err
	}
	artifactDigest, err := hex.DecodeString(digest)
	if err != nil || len(artifactDigest) != 32 {
		return fmt.Errorf("invalid artifact digest")
	}
	verifiers := make(map[string]*verify.Verifier)
	var lastErr error
	for i, att := range response.Attestations {
		if i >= 10 {
			break
		}
		var b bundle.Bundle
		if err := b.UnmarshalJSON(att.Bundle); err != nil {
			lastErr = err
			continue
		}
		content, err := b.VerificationContent()
		if err != nil {
			lastErr = err
			continue
		}
		cert := content.Certificate()
		if cert == nil || len(cert.Issuer.Organization) != 1 {
			continue
		}
		issuer := cert.Issuer.Organization[0]
		// This unverified value only selects between fixed, trusted authorities.
		verifier := verifiers[issuer]
		if verifier == nil {
			verifier, err = attestationVerifier(ctx, issuer)
			if err != nil {
				lastErr = err
				continue
			}
			verifiers[issuer] = verifier
		}
		result, err := verifier.Verify(&b, verify.NewPolicy(
			verify.WithArtifactDigest("sha256", artifactDigest), verify.WithCertificateIdentity(identity)))
		if err != nil {
			lastErr = err
			continue
		}
		if result.Statement == nil || result.Statement.PredicateType != "https://slsa.dev/provenance/v1" {
			lastErr = fmt.Errorf("attestation is not SLSA build provenance")
			continue
		}
		return nil
	}
	if lastErr != nil {
		return fmt.Errorf("no trusted release attestation: %w", lastErr)
	}
	return fmt.Errorf("release has no trusted build attestation")
}

func releaseIdentity(repo, tag, commit string) (verify.CertificateIdentity, error) {
	identity, err := verify.NewShortCertificateIdentity("https://token.actions.githubusercontent.com", "", "",
		`^https://github\.com/`+regexp.QuoteMeta(repo)+`/\.github/workflows/release\.yml@refs/(heads/master|tags/`+regexp.QuoteMeta(tag)+`)$`)
	identity.SourceRepositoryURI = "https://github.com/" + repo
	identity.SourceRepositoryDigest = commit
	identity.RunnerEnvironment = "github-hosted"
	return identity, err
}

func fetchAttestationJSON(ctx context.Context, client *http.Client, path string, dst any) error {
	resp, err := doWithGitHubAuthFallback(client, func(auth bool) (*http.Request, error) {
		req, err := newGitHubRequest(http.MethodGet, githubAPIURL(path), auth)
		if err != nil {
			return nil, err
		}
		req.Header.Set("Accept", "application/vnd.github+json")
		req.Header.Set("X-GitHub-Api-Version", "2022-11-28")
		return req.WithContext(ctx), nil
	})
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("GitHub API returned %d", resp.StatusCode)
	}
	return decodeJSONWithLimit(resp.Body, dst, maxReleaseMetadataBytes)
}

func attestationVerifier(ctx context.Context, issuer string) (*verify.Verifier, error) {
	opts := tuf.DefaultOptions().WithContext(ctx)
	opts.DisableLocalCache = true
	var requirements []verify.VerifierOption
	switch issuer {
	case "GitHub, Inc.":
		opts.Root = githubTUFRoot
		opts.RepositoryBaseURL = "https://tuf-repo.github.com"
		requirements = []verify.VerifierOption{verify.WithSignedTimestamps(1)}
	case "sigstore.dev":
		requirements = []verify.VerifierOption{verify.WithSignedCertificateTimestamps(1), verify.WithTransparencyLog(1), verify.WithObserverTimestamps(1)}
	default:
		return nil, fmt.Errorf("untrusted attestation issuer")
	}
	f := fetcher.NewDefaultFetcher()
	f.SetHTTPClient(&http.Client{Timeout: 15 * time.Second, Transport: attestationTransport{ctx}})
	opts.WithFetcher(f)
	client, err := tuf.New(opts)
	if err != nil {
		return nil, err
	}
	trusted, err := root.GetTrustedRoot(client)
	if err != nil {
		return nil, err
	}
	return verify.NewVerifier(trusted, requirements...)
}

type attestationTransport struct{ ctx context.Context }

func (t attestationTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	return http.DefaultTransport.RoundTrip(req.Clone(t.ctx))
}
