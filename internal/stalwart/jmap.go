package stalwart

import (
	"bytes"
	"context"
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

// DeploySieveScript uploads and activates a Sieve script for an account using
// JMAP SieveScript/set. The script is uploaded as a blob first, then set as
// active.
func (c *JMAPClient) DeploySieveScript(ctx context.Context, baseURL, adminToken, accountName, scriptName, scriptContent string) error {
	// Step 1: Upload blob.
	blobID, err := c.uploadBlob(ctx, baseURL, adminToken, accountName, []byte(scriptContent))
	if err != nil {
		return fmt.Errorf("upload sieve blob: %w", err)
	}

	// Step 2: Set the Sieve script (create or update) and activate it.
	request := map[string]any{
		"using": []string{"urn:ietf:params:jmap:core", "urn:ietf:params:jmap:mail", "urn:ietf:params:jmap:vacationresponse", "urn:ietf:params:jmap:managesieve"},
		"methodCalls": []any{
			[]any{"SieveScript/set", map[string]any{
				"accountId": accountName,
				"create": map[string]any{
					"script1": map[string]any{
						"name":     scriptName,
						"blobId":   blobID,
						"isActive": true,
					},
				},
				"onDestroyRemoveActive": true,
			}, "0"},
		},
	}

	_, err = c.jmapRequest(ctx, baseURL, adminToken, request)
	if err != nil {
		return fmt.Errorf("deploy sieve script: %w", err)
	}
	return nil
}

// DeleteSieveScript deactivates and removes a Sieve script by name.
// It first queries for the script ID, then destroys it.
func (c *JMAPClient) DeleteSieveScript(ctx context.Context, baseURL, adminToken, accountName, scriptName string) error {
	// Step 1: Query for the script ID.
	queryReq := map[string]any{
		"using": []string{"urn:ietf:params:jmap:core", "urn:ietf:params:jmap:mail", "urn:ietf:params:jmap:managesieve"},
		"methodCalls": []any{
			[]any{"SieveScript/query", map[string]any{
				"accountId": accountName,
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

	// Step 2: Destroy the scripts.
	destroyReq := map[string]any{
		"using": []string{"urn:ietf:params:jmap:core", "urn:ietf:params:jmap:mail", "urn:ietf:params:jmap:managesieve"},
		"methodCalls": []any{
			[]any{"SieveScript/set", map[string]any{
				"accountId":             accountName,
				"destroy":               scriptIDs,
				"onDestroyRemoveActive": true,
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
// using JMAP VacationResponse/set.
func (c *JMAPClient) SetVacationResponse(ctx context.Context, baseURL, adminToken, accountName string, vacation *VacationParams) error {
	update := map[string]any{
		"isEnabled": false,
	}
	if vacation != nil && vacation.Enabled {
		update["isEnabled"] = true
		update["subject"] = vacation.Subject
		update["textBody"] = vacation.Body
		if vacation.StartDate != nil {
			update["fromDate"] = *vacation.StartDate
		} else {
			update["fromDate"] = nil
		}
		if vacation.EndDate != nil {
			update["toDate"] = *vacation.EndDate
		} else {
			update["toDate"] = nil
		}
	}

	request := map[string]any{
		"using": []string{"urn:ietf:params:jmap:core", "urn:ietf:params:jmap:mail", "urn:ietf:params:jmap:vacationresponse"},
		"methodCalls": []any{
			[]any{"VacationResponse/set", map[string]any{
				"accountId": accountName,
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

// uploadBlob uploads a blob to the JMAP server and returns the blob ID.
func (c *JMAPClient) uploadBlob(ctx context.Context, baseURL, adminToken, accountName string, data []byte) (string, error) {
	url := fmt.Sprintf("%s/jmap/upload/%s/", baseURL, accountName)
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(data))
	if err != nil {
		return "", fmt.Errorf("upload blob request: %w", err)
	}
	req.SetBasicAuth("admin", adminToken)
	req.Header.Set("Content-Type", "application/octet-stream")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return "", fmt.Errorf("upload blob: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 300 {
		body, _ := io.ReadAll(resp.Body)
		return "", fmt.Errorf("upload blob: status %d: %s", resp.StatusCode, string(body))
	}

	var result struct {
		BlobID string `json:"blobId"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return "", fmt.Errorf("decode upload response: %w", err)
	}
	return result.BlobID, nil
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
