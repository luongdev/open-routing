// Command refagent is the v0.3 W6 reference agent client: a minimal WebSocket
// agent that proves the live transport + reservation lifecycle end to end. It
// connects to the agent gateway, optionally marks itself Ready, then auto-accepts
// every offer it is rung (echoing the offer's lease_token so the D5 fence passes)
// and optionally completes the call. It is a diagnostic/demo tool, not part of the
// serving plane — run it against a deployed cluster to watch the matcher work.
package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"github.com/coder/websocket"
	"github.com/google/uuid"
)

// inbound/outbound mirror the gateway envelope (internal/wsgateway) — duplicated
// here so the reference client has no dependency on server internals.
type clientCmd struct {
	ID          string `json:"id,omitempty"`
	Type        string `json:"type"`
	LastSeq     int64  `json:"last_server_seq,omitempty"`
	Reservation string `json:"reservation_id,omitempty"`
	LeaseToken  string `json:"lease_token,omitempty"`
}

type serverFrame struct {
	Type        string         `json:"type"`
	Seq         int64          `json:"server_seq,omitempty"`
	ReplyTo     string         `json:"reply_to,omitempty"`
	Status      string         `json:"status,omitempty"`
	SessionID   string         `json:"session_id,omitempty"`
	Reservation string         `json:"reservation_id,omitempty"`
	Payload     map[string]any `json:"payload,omitempty"`
	Reason      string         `json:"reason,omitempty"`
}

func main() { os.Exit(realMain()) }

func realMain() int {
	base := flag.String("url", "http://localhost:8080", "API base URL (http[s]://host:port)")
	org := flag.String("org", "", "org id (X-Org-Id)")
	agent := flag.String("agent", "", "agent id (X-Agent-Id)")
	markReady := flag.Bool("ready", false, "PATCH the agent to Ready before connecting")
	autoComplete := flag.Bool("complete", false, "send reservation.complete after a successful accept")
	flag.Parse()

	log := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelInfo}))
	if *org == "" || *agent == "" {
		log.Error("missing required flags", "need", "-org and -agent")
		return 2
	}

	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	if *markReady {
		if err := patchReady(ctx, *base, *org, *agent); err != nil {
			log.Warn("mark ready failed (set it manually if the agent isn't NotReady)", "err", err)
		} else {
			log.Info("agent marked Ready")
		}
	}

	if err := run(ctx, log, *base, *org, *agent, *autoComplete); err != nil && ctx.Err() == nil {
		log.Error("session ended", "err", err)
		return 1
	}
	return 0
}

func patchReady(ctx context.Context, base, org, agent string) error {
	url := fmt.Sprintf("%s/v1/orgs/%s/agents/%s/status", strings.TrimRight(base, "/"), org, agent)
	req, err := http.NewRequestWithContext(ctx, http.MethodPatch, url, strings.NewReader(`{"to":"Ready"}`))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Org-Id", org)
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return err
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("status %d", resp.StatusCode)
	}
	return nil
}

func run(ctx context.Context, log *slog.Logger, base, org, agent string, autoComplete bool) error {
	wsURL := "ws" + strings.TrimPrefix(strings.TrimRight(base, "/"), "http") + "/v1/agent/ws"
	conn, _, err := websocket.Dial(ctx, wsURL, &websocket.DialOptions{
		HTTPHeader: http.Header{"X-Org-Id": {org}, "X-Agent-Id": {agent}},
	})
	if err != nil {
		return fmt.Errorf("dial %s: %w", wsURL, err)
	}
	defer func() { _ = conn.Close(websocket.StatusNormalClosure, "bye") }()
	log.Info("connected", "url", wsURL, "agent", agent)

	// hello resumes the durable outbox from the start so we never miss a pending
	// offer that landed before we connected (or across a reconnect).
	if err := write(ctx, conn, clientCmd{Type: "hello", LastSeq: 0}); err != nil {
		return err
	}
	go heartbeat(ctx, conn)

	for {
		typ, data, rErr := conn.Read(ctx)
		if rErr != nil {
			return rErr
		}
		if typ != websocket.MessageText {
			continue
		}
		var f serverFrame
		if err := json.Unmarshal(data, &f); err != nil {
			log.Warn("bad frame", "err", err)
			continue
		}
		switch f.Type {
		case "welcome":
			log.Info("welcome", "session", f.SessionID)
		case "ack":
			log.Info("ack", "reply_to", f.ReplyTo, "status", f.Status, "reservation", f.Reservation)
			if autoComplete && f.Status == "accepted" && f.Reservation != "" {
				if err := write(ctx, conn, clientCmd{ID: uuid.NewString(), Type: "reservation.complete", Reservation: f.Reservation}); err != nil {
					return err
				}
				log.Info("sent complete", "reservation", f.Reservation)
			}
		case "error":
			log.Warn("server error", "reply_to", f.ReplyTo, "reason", f.Reason)
		case "reservation.offer":
			lease, _ := f.Payload["lease_token"].(string)
			res := f.Reservation
			if res == "" {
				res, _ = f.Payload["reservation_id"].(string)
			}
			log.Info("offer received → accepting", "reservation", res, "seq", f.Seq)
			if err := write(ctx, conn, clientCmd{ID: uuid.NewString(), Type: "reservation.accept", Reservation: res, LeaseToken: lease}); err != nil {
				return err
			}
		default:
			log.Info("frame", "type", f.Type, "seq", f.Seq)
		}
	}
}

func heartbeat(ctx context.Context, conn *websocket.Conn) {
	t := time.NewTicker(20 * time.Second)
	defer t.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-t.C:
			if err := write(ctx, conn, clientCmd{Type: "heartbeat"}); err != nil {
				return
			}
		}
	}
}

func write(ctx context.Context, conn *websocket.Conn, c clientCmd) error {
	b, err := json.Marshal(c)
	if err != nil {
		return err
	}
	wctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	return conn.Write(wctx, websocket.MessageText, b)
}
