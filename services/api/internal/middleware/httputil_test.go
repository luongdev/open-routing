package middleware

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
)

// TestWriteError_EmbedsRequestID_WhenCtxHasOne proves that when the
// RequestID middleware has already stored a UUIDv7 in ctx, WriteError
// embeds it as "request_id" in the JSON body. This satisfies D-35:
// every error response must carry the request_id from the same ctx.
func TestWriteError_EmbedsRequestID_WhenCtxHasOne(t *testing.T) {
	t.Parallel()

	// Mint a known UUIDv7 and store it in ctx the same way RequestID
	// middleware does.
	id := uuid.Must(uuid.NewV7())
	ctx := context.WithValue(context.Background(), requestIDCtxKey{}, id)

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/v1/orgs/x/_scaffold", nil)
	req = req.WithContext(ctx)

	WriteError(req.Context(), w, http.StatusBadRequest, "not_found", "no_such_scaffold")

	require.Equal(t, http.StatusBadRequest, w.Code)
	require.Equal(t, "application/json", w.Header().Get("Content-Type"))

	var body map[string]interface{}
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &body))

	require.Equal(t, "not_found", body["error"], "error field must match code")
	require.Equal(t, "no_such_scaffold", body["reason"], "reason field must match")

	// D-35: request_id must appear in the body with the same UUID stored by
	// the RequestID middleware.
	reqIDVal, ok := body["request_id"]
	require.True(t, ok, "response body must contain request_id field")
	require.Equal(t, id.String(), reqIDVal, "request_id must match the ctx value")
}

// TestWriteError_OmitsRequestID_WhenCtxLacksOne proves that when no
// RequestID middleware has run (bypass routes, tests that skip the
// middleware), WriteError still returns a valid JSON body but omits
// the "request_id" field (omitempty behaviour — the field is absent
// from the body, not present as "" or null). D-35 allows omission
// when the ctx does not carry one; the field is only embedded when
// present.
func TestWriteError_OmitsRequestID_WhenCtxLacksOne(t *testing.T) {
	t.Parallel()

	w := httptest.NewRecorder()
	// Plain background ctx — no RequestID middleware in the chain.
	WriteError(context.Background(), w, http.StatusInternalServerError, "internal", "something_failed")

	require.Equal(t, http.StatusInternalServerError, w.Code)
	require.Equal(t, "application/json", w.Header().Get("Content-Type"))

	var body map[string]interface{}
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &body))

	require.Equal(t, "internal", body["error"])
	require.Equal(t, "something_failed", body["reason"])

	// request_id MUST be absent (not "" and not null) when ctx lacks one.
	_, present := body["request_id"]
	require.False(t, present, "request_id must be absent from body when ctx has none")
}

// TestWriteJSON_Unchanged ensures the WriteJSON helper still works
// correctly after the WriteError signature change. This acts as a
// regression guard so the refactor does not accidentally touch WriteJSON.
func TestWriteJSON_Unchanged(t *testing.T) {
	t.Parallel()

	type payload struct {
		ID   string `json:"id"`
		Name string `json:"name"`
	}

	w := httptest.NewRecorder()
	WriteJSON(w, http.StatusOK, payload{ID: "abc", Name: "test"})

	require.Equal(t, http.StatusOK, w.Code)
	require.Equal(t, "application/json", w.Header().Get("Content-Type"))

	var got payload
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &got))
	require.Equal(t, "abc", got.ID)
	require.Equal(t, "test", got.Name)
}
