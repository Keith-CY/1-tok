# Coolify Testnet Deployment

Use Coolify's Docker Compose application flow against the public repo:

- Repository: `https://github.com/Keith-CY/1-tok`
- Initial branch: `feat/demo-ready-environment`
- Compose path: `deploy/coolify/testnet.compose.yaml`
- Project: `1-tok`
- Environment: `testnet`

After PR #201 merges, switch the tracked branch to `main` and keep auto-deploy enabled.

## Stack shape

The `testnet` stack is a full live demo environment. Treat these as standard services, not optional sidecars:

- `web`
- `api-gateway`
- `iam`
- `marketplace`
- `settlement`
- `settlement-reconciler`
- `execution`
- `risk`
- `notification`
- `carrier-daemon`
- `carrier-gateway`
- `remote-vps`
- `fnn`
- `fnn2`
- `provider-fnn`
- `fiber-adapter`
- `postgres`
- `redis`
- `nats`
- `bootstrap` as a one-shot migration/bootstrap job

`mock-fiber`, `mock-carrier`, and the CI-only E2E runners do not belong in the Coolify deployment.

## Public ingress

Attach Coolify-generated domains to:

- `web` on port `3000`
- `api-gateway` on port `8080`
- `settlement` on port `8083`

`iam` stays internal. The Next app talks to it over `IAM_BASE_URL=http://iam:8081`.

Set these public URLs in Coolify env:

- `PUBLIC_WEB_URL`
- `PUBLIC_API_BASE_URL`
- `PUBLIC_SETTLEMENT_BASE_URL`

`api-gateway` and `settlement` both use `CORS_ALLOWED_ORIGIN=${PUBLIC_WEB_URL}`.

## Required env and secrets

Core platform:

- `POSTGRES_PASSWORD`
- `ONE_TOK_EXECUTION_GATEWAY_TOKEN`
- `ONE_TOK_EXECUTION_EVENT_TOKEN`
- `ONE_TOK_SETTLEMENT_SERVICE_TOKEN`
- `FIBER_APP_ID`
- `FIBER_HMAC_SECRET`
- `FIBER_SECRET_KEY_PASSWORD`
- `FIBER_USDI_UDT_TYPE_SCRIPT_JSON`
- `CARRIER_SERVER_API_TOKEN`
- `CARRIER_GATEWAY_API_TOKEN`

Buyer deposit sweep:

- `BUYER_DEPOSIT_ENABLE=true`
- `BUYER_DEPOSIT_WALLET_MASTER_SEED`
- `BUYER_DEPOSIT_CKB_RPC_URL`
- `BUYER_DEPOSIT_CKB_NETWORK`
- `BUYER_DEPOSIT_TREASURY_ADDRESS`
- `BUYER_DEPOSIT_UDT_TYPE_SCRIPT_JSON` or `FIBER_USDI_UDT_TYPE_SCRIPT_JSON`
- `BUYER_DEPOSIT_UDT_CELL_DEP_TX_HASH`
- `BUYER_DEPOSIT_UDT_CELL_DEP_INDEX`
- optional: `BUYER_DEPOSIT_MIN_USDI`
- optional: `BUYER_DEPOSIT_CONFIRMATION_BLOCKS`

Carrier / remote execution:

- `CARRIER_E2E_REMOTE_AUTHORIZED_KEY`
- `CARRIER_REMOTE_PRIVATE_KEY_BASE64`
- optional: `OPENAI_API_KEY`, `OPENAI_CODEX_TOKEN`, `OPENAI_BASE_URL`
- optional build args override: `CARRIER_REPO_URL`, `CARRIER_REF`

Demo actors:

- `DEMO_BUYER_EMAIL`, `DEMO_BUYER_PASSWORD`
- `DEMO_PROVIDER_EMAIL`, `DEMO_PROVIDER_PASSWORD`
- `DEMO_OPS_EMAIL`, `DEMO_OPS_PASSWORD`
- optional stable names: `DEMO_*_NAME`, `DEMO_*_ORG_NAME`
- optional pinned org IDs: `DEMO_BUYER_ORG_ID`, `DEMO_PROVIDER_ORG_ID`, `DEMO_OPS_ORG_ID`

The org IDs are now optional. If omitted, the control plane resolves them by logging in with the configured demo accounts.

For the testnet demo, `api-gateway` defaults `RELEASE_USDI_E2E_CARRIER_BACKEND=codex`. When `OPENAI_BASE_URL` is set, `remote-vps` writes a global `~/.codex/config.toml` that points Codex at the custom OpenAI-compatible endpoint while keeping the demo on `gpt-5.4`.

## Operator flow

After the stack is healthy:

1. Log into `/ops`.
2. Use the `Prepare demo` action on the ops page.
3. Refresh `/ops` and confirm `Demo readiness` shows `ready`.
4. Run the live walkthrough from [docs/demo-runbook.md](../../docs/demo-runbook.md).

The same readiness verdict is available from `GET /api/v1/ops/demo/status`.

## Notes

- `carrier-daemon` and `carrier-gateway` build directly from the public `carrier` repo at `CARRIER_REF`; Coolify does not rely on the untracked local `./.deps/carrier` directory.
- `carrier-gateway` writes its SSH private key from `CARRIER_REMOTE_PRIVATE_KEY_BASE64` into `/keys/id_ed25519` at container startup, so no host-path secret mount is required.
- Use [docs/demo-environment.md](../../docs/demo-environment.md) and [docs/env.md](../../docs/env.md) as the environment contract for readiness thresholds and demo actor defaults.
