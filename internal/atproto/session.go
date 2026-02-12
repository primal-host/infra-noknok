package atproto

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
)

// createSessionRequest is the body for com.atproto.server.createSession.
type createSessionRequest struct {
	Identifier string `json:"identifier"`
	Password   string `json:"password"`
}

// createSessionResponse is the relevant fields from the createSession response.
type createSessionResponse struct {
	DID    string `json:"did"`
	Handle string `json:"handle"`
}

// Authenticate resolves a handle to a DID, finds the user's PDS, and calls
// createSession to verify the app password. Returns the authenticated DID and handle.
func Authenticate(handle, password string) (did string, resolvedHandle string, err error) {
	// Step 1: Resolve handle → DID.
	did, err = ResolveHandle(handle)
	if err != nil {
		return "", "", fmt.Errorf("authenticate: %w", err)
	}

	// Step 2: Resolve DID → PDS endpoint.
	pdsURL, err := ResolvePDS(did)
	if err != nil {
		return "", "", fmt.Errorf("authenticate: %w", err)
	}

	// Step 3: Call createSession on the user's PDS.
	body, err := json.Marshal(createSessionRequest{
		Identifier: handle,
		Password:   password,
	})
	if err != nil {
		return "", "", fmt.Errorf("authenticate: marshal: %w", err)
	}

	url := fmt.Sprintf("%s/xrpc/com.atproto.server.createSession", pdsURL)
	resp, err := httpClient.Post(url, "application/json", bytes.NewReader(body))
	if err != nil {
		return "", "", fmt.Errorf("authenticate: createSession: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return "", "", fmt.Errorf("authenticate: createSession returned status %d", resp.StatusCode)
	}

	var result createSessionResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return "", "", fmt.Errorf("authenticate: decode response: %w", err)
	}

	// The AT Protocol JWT tokens are discarded — only the DID matters.
	return result.DID, result.Handle, nil
}
