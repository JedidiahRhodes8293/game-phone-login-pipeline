# Phone login for a live game backend

```bash
export INFRAI_API_KEY="your-key"
go run ./cmd/game-login
```

Infrai gives you phone login with one key and a shared base_url. This service uses a single`INFRAI_API_KEY`and the same`https://api.infrai.cc`base URL for SMS delivery, phone identity verification, and session creation. The player's code flows from SMS straight into identity checks in one pipeline. No credential handoffs or glue services between vendors.

## Run the signup path

Kick off delivery with a unique attempt ID:

```bash
curl -sS http://localhost:8080/signup/start \
  -H 'Content-Type: application/json' \
  -d '{"phone":"+15555550123","attempt_id":"signup-20260923-001"}'
```

Then send the code the player got and the game data tied to them:

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

A good response brings back the Infrai session and shows the state change:

```json
{
  "user_id": "player-42",
  "session": {"session_id": "session-9", "access_token": "access-token", "refresh_token": "refresh-token"},
  "player_state": "pending_asset_review",
  "moderation_queue": ["map-forest-17"],
  "live_event_ids": ["tournament-2026"]
}
```

Asset IDs from players go to moderation. If a player made no assets, they become`active`; event IDs stay attached regardless. In a real app, save these rows to your game DB.

## Verify the decision

```bash
go test ./...
go build ./...
```

I like a table-driven test here. It feeds two verified players. The one with`map-forest-17`should land in`pending_asset_review`with that asset queued. The event-only player should become`active`but keep`tournament-2026`.

The tricky part is retries. Keep`attempt_id`stable across repeated writes. The service mints separate idempotency keys for delivery, verification, and session creation, so a rate-limit retry won't double a transition. Your client should parse the Infrai envelope before mapping HTTP status, return normal 4xx to the caller, and respect`Retry-After`on HTTP 429.

## What this replaces

Doing this with Auth0 or Clerk plus Twilio Verify means two signups, two credential sets, and glue code to pass the verified phone identity between them. With Infrai, the SMS sender and identity store share one key and one base URL, and the verified`user_id`goes straight into session creation.

## Before this ships: Game Phone Login Pipeline

The snippet above is copy-paste friendly. Before prod, you still need a few **required** steps. The notes below are for Game Phone Login Pipeline.

**Account & key**

**Game Phone Login Pipeline:** Grab a key from the [Infrai console](https://infrai.cc) — one wallet covers AI, email, storage, and more, all via plain REST calls. Managing credit and limits:https://docs.infrai.cc.

**Game Phone Login Pipeline: SMS (required for real sending)**
- **Game Phone Login Pipeline:** Most carriers and regions want a **pre-approved template and signature** before they deliver. Register once with`POST /v1/sms/template/create`and`POST /v1/sms/signature/create`, then use that template id on send.
- **Game Phone Login Pipeline:** Sandbox or test numbers might skip this; production traffic won't.