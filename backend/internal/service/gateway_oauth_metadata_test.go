package service

import (
	"encoding/json"
	"regexp"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestBuildOAuthMetadataUserID_FallbackWithoutAccountUUID(t *testing.T) {
	svc := &GatewayService{}

	parsed := &ParsedRequest{
		Model:          "claude-sonnet-4-5",
		Stream:         true,
		MetadataUserID: "",
		System:         nil,
		Messages:       nil,
	}

	account := &Account{
		ID:    123,
		Type:  AccountTypeOAuth,
		Extra: map[string]any{}, // intentionally missing account_uuid / claude_user_id
	}

	fp := &Fingerprint{ClientID: "deadbeef"} // should be used as device_id

	got := svc.buildOAuthMetadataUserID(parsed, account, fp)
	require.NotEmpty(t, got)

	// JSON format: {"device_id":"deadbeef","account_uuid":"","session_id":"<uuid>"}
	var j struct {
		DeviceID    string `json:"device_id"`
		AccountUUID string `json:"account_uuid"`
		SessionID   string `json:"session_id"`
	}
	require.NoError(t, json.Unmarshal([]byte(got), &j), "expected JSON user_id, got: %s", got)
	require.Equal(t, "deadbeef", j.DeviceID)
	require.Empty(t, j.AccountUUID)
	uuidRe := regexp.MustCompile(`^[a-f0-9-]{36}$`)
	require.True(t, uuidRe.MatchString(j.SessionID), "unexpected session_id: %s", j.SessionID)
}

func TestBuildOAuthMetadataUserID_UsesAccountUUIDWhenPresent(t *testing.T) {
	svc := &GatewayService{}

	parsed := &ParsedRequest{
		Model:          "claude-sonnet-4-5",
		Stream:         true,
		MetadataUserID: "",
	}

	account := &Account{
		ID:   123,
		Type: AccountTypeOAuth,
		Extra: map[string]any{
			"account_uuid":      "acc-uuid",
			"claude_user_id":    "clientid123",
			"anthropic_user_id": "",
		},
	}

	got := svc.buildOAuthMetadataUserID(parsed, account, nil)
	require.NotEmpty(t, got)

	// JSON format: {"device_id":"clientid123","account_uuid":"acc-uuid","session_id":"<uuid>"}
	var j struct {
		DeviceID    string `json:"device_id"`
		AccountUUID string `json:"account_uuid"`
		SessionID   string `json:"session_id"`
	}
	require.NoError(t, json.Unmarshal([]byte(got), &j), "expected JSON user_id, got: %s", got)
	require.Equal(t, "clientid123", j.DeviceID)
	require.Equal(t, "acc-uuid", j.AccountUUID)
	uuidRe := regexp.MustCompile(`^[a-f0-9-]{36}$`)
	require.True(t, uuidRe.MatchString(j.SessionID), "unexpected session_id: %s", j.SessionID)
}
