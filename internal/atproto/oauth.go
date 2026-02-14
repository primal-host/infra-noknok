package atproto

import (
	"context"
	"fmt"
	"net/url"
	"strings"

	"github.com/bluesky-social/indigo/atproto/atcrypto"
	"github.com/bluesky-social/indigo/atproto/auth/oauth"
	"github.com/bluesky-social/indigo/atproto/syntax"
)

// OAuthClient wraps the indigo OAuth ClientApp for AT Protocol login.
type OAuthClient struct {
	app *oauth.ClientApp
	cfg *oauth.ClientConfig
}

// NewOAuthClient creates an OAuth client configured as a confidential web app.
func NewOAuthClient(publicURL, privateKeyMultibase string, store oauth.ClientAuthStore) (*OAuthClient, error) {
	clientID := publicURL + "/.well-known/oauth-client-metadata"
	callbackURL := publicURL + "/oauth/callback"

	cfg := oauth.NewPublicConfig(clientID, callbackURL, []string{"atproto"})
	cfg.UserAgent = "noknok/0.4.0"

	privKey, err := atcrypto.ParsePrivateMultibase(privateKeyMultibase)
	if err != nil {
		return nil, fmt.Errorf("parse OAuth private key: %w", err)
	}
	if err := cfg.SetClientSecret(privKey, "noknok-1"); err != nil {
		return nil, fmt.Errorf("set client secret: %w", err)
	}

	app := oauth.NewClientApp(&cfg, store)
	return &OAuthClient{app: app, cfg: &cfg}, nil
}

// StartLogin begins the OAuth flow for the given handle, returning the
// authorization URL the user should be redirected to.
func (c *OAuthClient) StartLogin(ctx context.Context, handle string) (string, error) {
	return c.app.StartAuthFlow(ctx, handle)
}

// HandleCallback processes the OAuth callback parameters and returns
// the authenticated DID and handle.
func (c *OAuthClient) HandleCallback(ctx context.Context, params url.Values) (string, string, error) {
	sess, err := c.app.ProcessCallback(ctx, params)
	if err != nil {
		return "", "", err
	}

	// Look up handle from DID (should be cached from ProcessCallback's lookup).
	ident, err := c.app.Dir.LookupDID(ctx, sess.AccountDID)
	if err != nil {
		return sess.AccountDID.String(), "", nil
	}
	return sess.AccountDID.String(), ident.Handle.String(), nil
}

// ClientMetadata returns the OAuth client metadata document.
func (c *OAuthClient) ClientMetadata() oauth.ClientMetadata {
	m := c.cfg.ClientMetadata()
	// Confidential clients must set JWKS URI after the fact.
	jwksURI := c.cfg.ClientID[:len(c.cfg.ClientID)-len("/.well-known/oauth-client-metadata")] + "/oauth/jwks.json"
	m.JWKSURI = &jwksURI
	name := "noknok"
	m.ClientName = &name
	return m
}

// PublicJWKS returns the public key set for client assertion verification.
func (c *OAuthClient) PublicJWKS() oauth.JWKS {
	return c.cfg.PublicJWKS()
}

// ResolveHandle resolves a handle to a DID and canonical handle.
// Bare names (no dot) default to .bsky.social.
func (c *OAuthClient) ResolveHandle(ctx context.Context, handle string) (string, string, error) {
	if !strings.Contains(handle, ".") {
		handle += ".bsky.social"
	}
	hdl, err := syntax.ParseHandle(handle)
	if err != nil {
		return "", "", fmt.Errorf("invalid handle: %w", err)
	}
	ident, err := c.app.Dir.LookupHandle(ctx, hdl)
	if err != nil {
		return "", "", fmt.Errorf("resolve handle %s: %w", handle, err)
	}
	return ident.DID.String(), ident.Handle.String(), nil
}
