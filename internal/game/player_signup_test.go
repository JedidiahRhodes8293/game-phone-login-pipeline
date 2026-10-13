package game

import (
	"context"
	"reflect"
	"testing"

	"github.com/infrai-examples/game-phone-login/internal/infrai"
)

type fakeIdentity struct {
	verifiedUserID string
	session        infrai.Session
}

func (f fakeIdentity) SendOTP(context.Context, string, string) error { return nil }
func (f fakeIdentity) VerifyPhone(context.Context, string, string, string) (infrai.VerifyResult, error) {
	return infrai.VerifyResult{UserID: f.verifiedUserID}, nil
}
func (f fakeIdentity) CreateSession(context.Context, string, string) (infrai.Session, error) {
	return f.session, nil
}

func TestCompleteRoutesPlayerAssets(t *testing.T) {
	tests := []struct {
		name      string
		assets    []string
		wantState string
		wantQueue []string
	}{
		{name: "generated maps wait for review", assets: []string{"map-forest-17", "skin-neon-4"}, wantState: "pending_asset_review", wantQueue: []string{"map-forest-17", "skin-neon-4"}},
		{name: "event-only player is active", wantState: "active", wantQueue: []string{}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			service := NewSignupService(fakeIdentity{
				verifiedUserID: "player-42",
				session:        infrai.Session{SessionID: "session-9"},
			})
			got, err := service.Complete(context.Background(), SignupInput{
				AttemptID: "attempt-7",
				Code:      "123456",
				Player: PlayerProfile{
					Phone:           "+15555550123",
					GeneratedAssets: tt.assets,
					LiveEventIDs:    []string{"tournament-2026"},
				},
			})
			if err != nil {
				t.Fatal(err)
			}
			if got.PlayerState != tt.wantState {
				t.Fatalf("state = %q, want %q", got.PlayerState, tt.wantState)
			}
			if !reflect.DeepEqual(got.ModerationQueue, tt.wantQueue) {
				t.Fatalf("queue = %#v, want %#v", got.ModerationQueue, tt.wantQueue)
			}
			if !reflect.DeepEqual(got.LiveEventIDs, []string{"tournament-2026"}) {
				t.Fatalf("live events were not preserved: %#v", got.LiveEventIDs)
			}
		})
	}
}
