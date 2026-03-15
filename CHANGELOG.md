# Changelog

## v1.0.0 (alpha)

### Features
- **Core Marketplace**: RFQ → Bid → Award → Order lifecycle with milestone settlement
- **Listing Search**: Full-text, category, tag, and price range filtering
- **Provider Search**: Capability, tier, and rating filtering
- **Provider Ratings**: 1-5 stars with duplicate prevention and computed averages
- **RFQ Messaging**: Buyer-provider communication during bid phase
- **Order Messaging**: Thread-based messaging on active orders
- **Carrier Integration**: Async execution protocol with job state machine and heartbeat
- **Notifications**: 8 event types with webhook delivery (HMAC-SHA256 signed)
- **Discord Bot**: 7 slash commands (/listings, /rfq-status, /order-status, /bids, /stats, /rfq-create, /award)
- **CSV Export**: Download orders and disputes as CSV
- **Marketplace Stats**: Aggregate statistics endpoint
- **Order Timeline**: Chronological audit trail
- **Batch Status**: Multi-order status query
- **Budget Summary**: Per-milestone budget utilization

### Anti-Fraud
- **Layer 2**: HMAC-SHA256 usage proof signatures
- **Layer 3**: Settlement reconciliation with anomaly detection

### Security
- IAM actor caching per request
- Session token hashing (SHA-256)
- Auth enforcement on rating + messaging endpoints
- API key authentication middleware
- Security headers (HSTS, CSP, X-Frame-Options)
- Request timeout middleware
- io.ReadAll size limits

### Infrastructure
- 50 API endpoints with pagination
- 26 Go packages
- 16 httputil middleware modules (97.8% coverage)
- Input validation with per-field error details
- Gzip compression middleware
- Request ID tracing
- /livez and /readyz health endpoints
- OpenAPI 3.0 spec
- CI: Postgres + NATS + Redis containers

### Metrics
- 264 commits
- 15.0k lines Go code
- 25.5k lines Go tests (1.7:1 test ratio)
- 2.2k lines TypeScript
- 52 API endpoints
- 8 Discord commands
- 27 Go packages (5 at 100% coverage)
- 16 httputil middleware modules (97.8% coverage)
- 87.7% CI statement coverage
- 99.38% function coverage
