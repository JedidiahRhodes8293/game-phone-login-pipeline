package game

import (
	"context"
	"fmt"

	"github.com/infrai-examples/game-phone-login/internal/infrai"
)

type PlayerProfile struct {
	Phone           string   `json:"phone"`
	DisplayName     string   `json:"display_name"`
	GeneratedAssets []string `json:"generated_assets"`
	LiveEventIDs    []string `json:"live_event_ids"`
}

type SignupInput struct {
	AttemptID string        `json:"attempt_id"`
	Code      string        `json:"code"`
	Player    PlayerProfile `json:"player"`
}

type SignupResult struct {
	UserID          string         `json:"user_id"`
	Session         infrai.Session `json:"session"`
	PlayerState     string         `json:"player_state"`
	ModerationQueue []string       `json:"moderation_queue"`
	LiveEventIDs    []string       `json:"live_event_ids"`
}

type IdentityGateway interface {
	SendOTP(context.Context, string, string) error
	VerifyPhone(context.Context, string, string, string) (infrai.VerifyResult, error)
	CreateSession(context.Context, string, string) (infrai.Session, error)
}

type SignupService struct {
	identity IdentityGateway
}

func NewSignupService(identity IdentityGateway) *SignupService {
	return &SignupService{identity: identity}
}

func (s *SignupService) Start(ctx context.Context, phone, attemptID string) error {
	return s.identity.SendOTP(ctx, phone, "signup:"+attemptID)
}

func (s *SignupService) Complete(ctx context.Context, input SignupInput) (SignupResult, error) {
	verified, err := s.identity.VerifyPhone(ctx, input.Player.Phone, input.Code, "verify:"+input.AttemptID)
	if err != nil {
		return SignupResult{}, err
	}
	if verified.UserID == "" {
		return SignupResult{}, fmt.Errorf("verified identity has no user_id")
	}
	session, err := s.identity.CreateSession(ctx, verified.UserID, "session:"+input.AttemptID)
	if err != nil {
		return SignupResult{}, err
	}

	state := "active"
	queue := make([]string, len(input.Player.GeneratedAssets))
	copy(queue, input.Player.GeneratedAssets)
	if len(queue) > 0 {
		state = "pending_asset_review"
	}
	return SignupResult{
		UserID:          verified.UserID,
		Session:         session,
		PlayerState:     state,
		ModerationQueue: queue,
		LiveEventIDs:    append([]string(nil), input.Player.LiveEventIDs...),
	}, nil
}
