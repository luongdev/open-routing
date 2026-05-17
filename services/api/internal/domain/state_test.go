package domain

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestIsRoutable(t *testing.T) {
	cases := []struct {
		name string
		in   AgentStateInputs
		want bool
	}{
		{"Ready returns true", AgentStateInputs{Status: "Ready"}, true},
		{"Ready ignores BreakReasonRoutable=false", AgentStateInputs{Status: "Ready", BreakReasonRoutable: false}, true},
		{"Break+routable=true returns true", AgentStateInputs{Status: "Break", BreakReasonRoutable: true}, true},
		{"Break+routable=false returns false", AgentStateInputs{Status: "Break", BreakReasonRoutable: false}, false},
		{"NotReady returns false", AgentStateInputs{Status: "NotReady"}, false},
		{"Engaged returns false", AgentStateInputs{Status: "Engaged"}, false},
		{"WrapUp returns false", AgentStateInputs{Status: "WrapUp"}, false},
		{"Offline returns false", AgentStateInputs{Status: "Offline"}, false},
		{"unknown status returns false (default deny)", AgentStateInputs{Status: "garbage"}, false},
		{"empty status returns false (default deny)", AgentStateInputs{Status: ""}, false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			require.Equal(t, tc.want, IsRoutable(tc.in))
		})
	}
}
