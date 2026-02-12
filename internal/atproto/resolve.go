package atproto

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

var httpClient = &http.Client{Timeout: 10 * time.Second}

// ResolveHandle resolves an AT Protocol handle to a DID.
// GET https://<handle>/.well-known/atproto-did
func ResolveHandle(handle string) (string, error) {
	url := fmt.Sprintf("https://%s/.well-known/atproto-did", handle)
	resp, err := httpClient.Get(url)
	if err != nil {
		return "", fmt.Errorf("resolve handle %s: %w", handle, err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("resolve handle %s: status %d", handle, resp.StatusCode)
	}

	body, err := io.ReadAll(io.LimitReader(resp.Body, 4096))
	if err != nil {
		return "", fmt.Errorf("resolve handle %s: read body: %w", handle, err)
	}

	did := strings.TrimSpace(string(body))
	if !strings.HasPrefix(did, "did:") {
		return "", fmt.Errorf("resolve handle %s: invalid DID: %q", handle, did)
	}
	return did, nil
}

// plcDocument is the minimal structure of a PLC directory document.
type plcDocument struct {
	Service []plcService `json:"service"`
}

type plcService struct {
	ID              string `json:"id"`
	Type            string `json:"type"`
	ServiceEndpoint string `json:"serviceEndpoint"`
}

// ResolvePDS resolves a DID to its PDS service endpoint URL.
// GET https://plc.directory/<did>
func ResolvePDS(did string) (string, error) {
	url := fmt.Sprintf("https://plc.directory/%s", did)
	resp, err := httpClient.Get(url)
	if err != nil {
		return "", fmt.Errorf("resolve PDS for %s: %w", did, err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("resolve PDS for %s: status %d", did, resp.StatusCode)
	}

	var doc plcDocument
	if err := json.NewDecoder(resp.Body).Decode(&doc); err != nil {
		return "", fmt.Errorf("resolve PDS for %s: decode: %w", did, err)
	}

	for _, svc := range doc.Service {
		if svc.ID == "#atproto_pds" {
			return svc.ServiceEndpoint, nil
		}
	}
	return "", fmt.Errorf("resolve PDS for %s: no #atproto_pds service found", did)
}
