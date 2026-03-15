# Changelog

## v1.0.0 (alpha)

### Features
- **Core Marketplace**: RFQ → Bid → Award → Order lifecycle with milestone settlement
- **Listing Search**: Full-text, category, tag, and price range filtering + sorting
- **Provider Search**: Capability, tier, and rating filtering + leaderboard
- **Provider Ratings**: 1-5 stars with duplicate prevention and computed averages
- **Provider Vetting**: Ops approval workflow for new providers
- **RFQ Messaging**: Buyer-provider communication during bid phase
- **Order Messaging**: Thread-based messaging on active orders
- **Carrier Integration**: Async execution protocol with job state machine, heartbeat, callbacks, profiles, evidence
- **Notifications**: 8 event types with webhook delivery (HMAC-SHA256 signed)
- **Discord Bot**: 8 slash commands (/listings, /rfq-status, /order-status, /bids, /stats, /rfq-create, /award, /leaderboard)
- **CSV Export**: Download orders and disputes as CSV
- **Marketplace Stats**: Aggregate statistics + provider leaderboard
- **Order Timeline**: Chronological audit trail
- **Budget Wall**: Top-up to resume paused milestones

### Anti-Fraud
- **Layer 2**: HMAC-SHA256 usage proof signatures
- **Layer 3**: Settlement reconciliation with anomaly detection

### Security
- Auth enforcement on all mutation + sensitive endpoints
- Carrier routes require execution token
- Rating requires order participant verification
- Messages require authenticated actor
- Webhooks/notifications require auth
- API key authentication middleware
- Security headers (HSTS, CSP, X-Frame-Options)
- Request timeout middleware
- io.ReadAll size limits

### Frontend
- 14 portal pages (Buyer, Provider, Ops)
- Listing search, RFQ creation, order detail with progress bars
- Provider carrier management, listing management, RFQ bidding
- Ops dispute arbitration, provider vetting

### Infrastructure
- 64 API endpoints with pagination, filtering, sorting
- 28 Go packages (5 at 100% coverage)
- 16 httputil middleware modules (97.8% coverage)
- Input validation with per-field error details
- Structured JSON logging
- Graceful shutdown with signal handling
- Centralized config from environment variables
- OpenAPI 3.0 spec
- Docker Compose deployment topology
- CI: Postgres + NATS + Redis containers

### Metrics
- 15.7k lines Go code
- 26k lines Go tests (1.7:1 test ratio)
- 2.5k lines TypeScript
- 64 API endpoints
- 8 Discord commands
- 28 Go packages
- 16 httputil middleware modules
