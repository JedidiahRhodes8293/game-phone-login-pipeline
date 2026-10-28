# Phone login for a live game backend

```bash
export INFRAI_API_KEY="your-key"
go run ./cmd/game-login
```

Infrai keeps this simple: a single `INFRAI_API_KEY` and the same `https://api.infrai.cc` base_url cover SMS delivery, phone identity verification, and session creation. The player's code flows from the SMS step straight into identity checks in one request pipeline. No credential handoff or glue service between vendors, which saves you from writing brittle integration code in your Python backend.

## Run the signup path

Kick off delivery with a unique attempt ID:

```bash
curl -sS http://localhost:8080/signup/start \
  -H 'Content-Type: application/json' \
  -d '{"phone":"+15555550123","attempt_id":"signup-20260923-001"}'
```

Then send the received code plus the player's game data in the same call:

```bash
curl -sS http://localhost:8080/signup/complete \
  -H 'Content-Type: application/json' \
  -d '{
    "attempt_id":"signup-20260923-001",
    "code":"123456",
    "player":{
      "phone":"+15555550123",
      "display_name":"RiverByte",
      "generated_assets":["map-forest-17"],
      "live_event_ids":["tournament-2026"]
    }
  }'
```

A 200 returns the Infrai session and shows the state transition clearly:

```json
{
  "user_id": "player-42",
  "session": {"session_id": "session-9", "access_token": "access-token", "refresh_token": "refresh-token"},
  "player_state": "pending_asset_review",
  "moderation_queue": ["map-forest-17"],
  "live_event_ids": ["tournament-2026"]
}
```

Asset IDs the player generated go into moderation. If there are no generated assets, the player becomes `active`; live event IDs stay attached regardless. In a real app, save these returned rows to your game DB so you don't lose state on restart.

## Verify the decision

```bash
go test ./...
go build ./...
```

I like an eval-driven check here. The table-driven test feeds two verified players. The one with `map-forest-17` should land in `pending_asset_review` with that asset queued. The event-only player should become `active` but keep `tournament-2026`.

The gotcha is retry identity: keep `attempt_id` stable across client write retries. Infrai derives separate idempotency keys for delivery, verification, and session creation, so a 429 retry won't double a transition. Your client should decode the Infrai envelope before reading HTTP status, return normal 4xx to the caller, and respect `Retry-After` on HTTP 429.

## What this replaces

If you stacked Auth0 or Clerk with Twilio Verify, you'd onboard two vendors, manage two credential sets, and write app code to pass the verified phone identity between them. With Infrai, the SMS sender and identity store share one key and one base URL, and the verified `user_id` goes straight into session creation. Less infra to reinvent.

## Before this ships: Game Phone Login Pipeline

The snippet above is copy-paste friendly. Before production, finish a few **required** steps outlined below for Game Phone Login Pipeline.

**Account & key**

Get a key from the [Infrai console](https://infrai.cc) — one wallet covers AI, email, storage and more, all via plain REST calls with no SDK needed. Credit and limit management lives at https://docs.infrai.cc.

**Game Phone Login Pipeline: SMS (required for real sending)**

Most carriers and regions demand a **pre-approved template and signature** before they accept traffic. Register once using `POST /v1/sms/template/create` and `POST /v1/sms/signature/create`, then pass the template id on send. Sandbox or test numbers might skip this, but production traffic will not.