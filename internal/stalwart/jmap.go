package stalwart

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
)

// JMAPClient provides JMAP protocol access to Stalwart for Sieve scripts and
// vacation auto-reply management.
type JMAPClient struct {
	httpClient *http.Client
}

// NewJMAPClient creates a new JMAPClient.
func NewJMAPClient() *JMAPClient {
	return &JMAPClient{httpClient: &http.Client{}}
}

// sieveCapabilities are the JMAP capabilities needed for Sieve script operations.
var sieveCapabilities = []string{
	"urn:ietf:params:jmap:core",
	"urn:ietf:params:jmap:mail",
	"urn:ietf:params:jmap:sieve",
}

// DeploySieveScript uploads and activates a Sieve script for an account using
// JMAP Blob/upload + SieveScript/set. The accountID must be a pre-resolved
// Crockford base32 encoded principal ID.
func (c *JMAPClient) DeploySieveScript(ctx context.Context, baseURL, adminToken, accountID, scriptName, scriptContent string) error {
	// Step 1: Upload blob via JMAP Blob/upload method.
	blobID, err := c.uploadBlobJMAP(ctx, baseURL, adminToken, accountID, []byte(scriptContent))
	if err != nil {
		return fmt.Errorf("upload sieve blob: %w", err)
	}

	// Step 2: Query for existing script by name.
	queryReq := map[string]any{
		"using": sieveCapabilities,
		"methodCalls": []any{
			[]any{"SieveScript/query", map[string]any{
				"accountId": accountID,
				"filter": map[string]any{
					"name": scriptName,
				},
			}, "0"},
		},
	}

	queryResp, err := c.jmapRequest(ctx, baseURL, adminToken, queryReq)
	if err != nil {
		return fmt.Errorf("query sieve scripts: %w", err)
	}

	existingIDs := extractQueryIDs(queryResp)

	// Step 3: Create or update the script and activate it.
	if len(existingIDs) > 0 {
		// Update existing script with new blob and activate it.
		scriptID := existingIDs[0]
		request := map[string]any{
			"using": sieveCapabilities,
			"methodCalls": []any{
				[]any{"SieveScript/set", map[string]any{
					"accountId": accountID,
					"update": map[string]any{
						scriptID: map[string]any{
							"blobId": blobID,
						},
					},
					"onSuccessActivateScript": scriptID,
				}, "0"},
			},
		}
		_, err = c.jmapRequest(ctx, baseURL, adminToken, request)
	} else {
		// Create new script and activate it.
		request := map[string]any{
			"using": sieveCapabilities,
			"methodCalls": []any{
				[]any{"SieveScript/set", map[string]any{
					"accountId": accountID,
					"create": map[string]any{
						"script1": map[string]any{
							"name":   scriptName,
							"blobId": blobID,
						},
					},
					"onSuccessActivateScript": "#script1",
				}, "0"},
			},
		}
		_, err = c.jmapRequest(ctx, baseURL, adminToken, request)
	}

	if err != nil {
		return fmt.Errorf("deploy sieve script: %w", err)
	}
	return nil
}

// DeleteSieveScript deactivates and removes a Sieve script by name.
// The accountID must be a pre-resolved Crockford base32 encoded principal ID.
func (c *JMAPClient) DeleteSieveScript(ctx context.Context, baseURL, adminToken, accountID, scriptName string) error {
	// Step 1: Query for the script ID.
	queryReq := map[string]any{
		"using": sieveCapabilities,
		"methodCalls": []any{
			[]any{"SieveScript/query", map[string]any{
				"accountId": accountID,
				"filter": map[string]any{
					"name": scriptName,
				},
			}, "0"},
		},
	}

	queryResp, err := c.jmapRequest(ctx, baseURL, adminToken, queryReq)
	if err != nil {
		return fmt.Errorf("query sieve scripts: %w", err)
	}

	scriptIDs := extractQueryIDs(queryResp)
	if len(scriptIDs) == 0 {
		return nil // Nothing to delete.
	}

	// Step 2: Deactivate and destroy in one request.
	destroyReq := map[string]any{
		"using": sieveCapabilities,
		"methodCalls": []any{
			[]any{"SieveScript/set", map[string]any{
				"accountId":               accountID,
				"onSuccessActivateScript": nil,
				"destroy":                 scriptIDs,
			}, "0"},
		},
	}

	_, err = c.jmapRequest(ctx, baseURL, adminToken, destroyReq)
	if err != nil {
		return fmt.Errorf("destroy sieve script: %w", err)
	}
	return nil
}

// SetVacationResponse sets or clears the vacation auto-reply for an account
// using JMAP VacationResponse/set. The accountID must be a pre-resolved
// Crockford base32 encoded principal ID.
func (c *JMAPClient) SetVacationResponse(ctx context.Context, baseURL, adminToken, accountID string, vacation *VacationParams) error {
	update := map[string]any{
		"isEnabled": false,
	}
	if vacation != nil && vacation.Enabled {
		update["isEnabled"] = true
		update["subject"] = vacation.Subject
		update["textBody"] = vacation.Body
		// Only include dates when set — Stalwart rejects explicit null values.
		if vacation.StartDate != nil {
			update["fromDate"] = *vacation.StartDate
		}
		if vacation.EndDate != nil {
			update["toDate"] = *vacation.EndDate
		}
	}

	request := map[string]any{
		"using": []string{"urn:ietf:params:jmap:core", "urn:ietf:params:jmap:mail", "urn:ietf:params:jmap:vacationresponse"},
		"methodCalls": []any{
			[]any{"VacationResponse/set", map[string]any{
				"accountId": accountID,
				"update": map[string]any{
					"singleton": update,
				},
			}, "0"},
		},
	}

	_, err := c.jmapRequest(ctx, baseURL, adminToken, request)
	if err != nil {
		return fmt.Errorf("set vacation response: %w", err)
	}
	return nil
}

// uploadBlobJMAP uploads a blob via the JMAP Blob/upload method (not REST),
// which works with Crockford base32 account IDs.
func (c *JMAPClient) uploadBlobJMAP(ctx context.Context, baseURL, adminToken, accountID string, data []byte) (string, error) {
	encoded := base64.StdEncoding.EncodeToString(data)

	request := map[string]any{
		"using": []string{"urn:ietf:params:jmap:core", "urn:ietf:params:jmap:blob"},
		"methodCalls": []any{
			[]any{"Blob/upload", map[string]any{
				"accountId": accountID,
				"create": map[string]any{
					"blob1": map[string]any{
						"data": []map[string]any{
							{"data:asBase64": encoded},
						},
						"type": "application/sieve",
					},
				},
			}, "0"},
		},
	}

	resp, err := c.jmapRequest(ctx, baseURL, adminToken, request)
	if err != nil {
		return "", fmt.Errorf("blob upload: %w", err)
	}

	blobID := extractCreatedBlobID(resp)
	if blobID == "" {
		return "", fmt.Errorf("blob upload: no blobId in response")
	}
	return blobID, nil
}

// jmapRequest sends a JMAP request and returns the parsed response.
func (c *JMAPClient) jmapRequest(ctx context.Context, baseURL, adminToken string, request any) (map[string]any, error) {
	body, err := json.Marshal(request)
	if err != nil {
		return nil, fmt.Errorf("marshal jmap request: %w", err)
	}

	url := fmt.Sprintf("%s/jmap", baseURL)
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("jmap request: %w", err)
	}
	req.SetBasicAuth("admin", adminToken)
	req.Header.Set("Content-Type", "application/json")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("jmap request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 300 {
		respBody, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("jmap request: status %d: %s", resp.StatusCode, string(respBody))
	}

	var result map[string]any
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("decode jmap response: %w", err)
	}
	return result, nil
}

// extractQueryIDs extracts IDs from a JMAP query response.
func extractQueryIDs(resp map[string]any) []string {
	calls, ok := resp["methodResponses"].([]any)
	if !ok || len(calls) == 0 {
		return nil
	}
	call, ok := calls[0].([]any)
	if !ok || len(call) < 2 {
		return nil
	}
	args, ok := call[1].(map[string]any)
	if !ok {
		return nil
	}
	ids, ok := args["ids"].([]any)
	if !ok {
		return nil
	}
	result := make([]string, 0, len(ids))
	for _, id := range ids {
		if s, ok := id.(string); ok {
			result = append(result, s)
		}
	}
	return result
}

// extractCreatedBlobID extracts the blobId from a Blob/upload response.
func extractCreatedBlobID(resp map[string]any) string {
	calls, ok := resp["methodResponses"].([]any)
	if !ok || len(calls) == 0 {
		return ""
	}
	call, ok := calls[0].([]any)
	if !ok || len(call) < 2 {
		return ""
	}
	args, ok := call[1].(map[string]any)
	if !ok {
		return ""
	}
	created, ok := args["created"].(map[string]any)
	if !ok {
		return ""
	}
	blob, ok := created["blob1"].(map[string]any)
	if !ok {
		return ""
	}
	// Stalwart returns "id" for Blob/upload (not "blobId").
	blobID, _ := blob["id"].(string)
	return blobID
}
