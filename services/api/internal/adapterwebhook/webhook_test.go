package adapterwebhook

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/luongdev/open-routing/services/api/internal/adapter"
)

type fakeSink struct {
	got   *adapter.AssignmentEvent
	err   error
	calls int
}

func (f *fakeSink) OnAssignmentEvent(_ context.Context, ev adapter.AssignmentEvent) error {
	f.calls++
	f.got = &ev
	return f.err
}

const secret = "shh-test-secret"

var fixedNow = func() time.Time { return time.Unix(1_780_000_000, 0) }

func post(t *testing.T, h http.HandlerFunc, ts, sig, body string) *httptest.ResponseRecorder {
	t.Helper()
	req := httptest.NewRequest(http.MethodPost, "/v1/adapter/assignment-events", strings.NewReader(body))
	if ts != "" {
		req.Header.Set(tsHeader, ts)
	}
	if sig != "" {
		req.Header.Set(sigHeader, sig)
	}
	rec := httptest.NewRecorder()
	h(rec, req)
	return rec
}

func TestWebhook_ValidSignatureDelivers(t *testing.T) {
	sink := &fakeSink{}
	h := Handler(sink, secret, nil, fixedNow)
	body := `{"type":"caller_abandoned","handle":"room-1","reservation_id":"r1","correlation_id":"c1","reason":"hangup"}`
	ts, sig := Sign(secret, fixedNow(), []byte(body))
	rec := post(t, h, ts, sig, body)
	if rec.Code != http.StatusOK {
		t.Fatalf("code = %d, want 200", rec.Code)
	}
	if sink.calls != 1 || sink.got.Type != adapter.EventCallerLeft || string(sink.got.Handle) != "room-1" || sink.got.ReservationID != "r1" {
		t.Fatalf("sink got %+v (calls=%d)", sink.got, sink.calls)
	}
}

func TestWebhook_BadSignatureRejected(t *testing.T) {
	sink := &fakeSink{}
	h := Handler(sink, secret, nil, fixedNow)
	body := `{"type":"failed","handle":"room-1","reservation_id":"r1"}`
	ts, _ := Sign(secret, fixedNow(), []byte(body))
	rec := post(t, h, ts, "deadbeef", body)
	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("code = %d, want 401", rec.Code)
	}
	if sink.calls != 0 {
		t.Fatal("sink must not be called on a bad signature")
	}
}

func TestWebhook_TamperedBodyRejected(t *testing.T) {
	sink := &fakeSink{}
	h := Handler(sink, secret, nil, fixedNow)
	signed := `{"type":"failed","handle":"room-1","reservation_id":"r1"}`
	ts, sig := Sign(secret, fixedNow(), []byte(signed))
	// Same signature, different body → HMAC mismatch.
	rec := post(t, h, ts, sig, `{"type":"completed","handle":"room-1","reservation_id":"r1"}`)
	if rec.Code != http.StatusUnauthorized || sink.calls != 0 {
		t.Fatalf("tampered body must be rejected: code=%d calls=%d", rec.Code, sink.calls)
	}
}

func TestWebhook_StaleTimestampRejected(t *testing.T) {
	sink := &fakeSink{}
	h := Handler(sink, secret, nil, fixedNow)
	body := `{"type":"failed","handle":"room-1","reservation_id":"r1"}`
	old := time.Unix(fixedNow().Unix()-int64((10*time.Minute).Seconds()), 0)
	ts, sig := Sign(secret, old, []byte(body))
	rec := post(t, h, ts, sig, body)
	if rec.Code != http.StatusUnauthorized || sink.calls != 0 {
		t.Fatalf("stale timestamp must be rejected: code=%d calls=%d", rec.Code, sink.calls)
	}
}

func TestWebhook_BadJSONRejected(t *testing.T) {
	sink := &fakeSink{}
	h := Handler(sink, secret, nil, fixedNow)
	body := `{not json`
	ts, sig := Sign(secret, fixedNow(), []byte(body))
	rec := post(t, h, ts, sig, body)
	if rec.Code != http.StatusBadRequest || sink.calls != 0 {
		t.Fatalf("bad json must be 400: code=%d calls=%d", rec.Code, sink.calls)
	}
}

func TestWebhook_SinkErrorIs500(t *testing.T) {
	sink := &fakeSink{err: errors.New("transient")}
	h := Handler(sink, secret, nil, fixedNow)
	body := `{"type":"disconnected","handle":"room-1","reservation_id":"r1"}`
	ts, sig := Sign(secret, fixedNow(), []byte(body))
	rec := post(t, h, ts, sig, body)
	if rec.Code != http.StatusInternalServerError {
		t.Fatalf("sink error must surface as 500 (adapter retries), got %d", rec.Code)
	}
}
