CREATE SEQUENCE IF NOT EXISTS order_seq START 1;
CREATE SEQUENCE IF NOT EXISTS rfq_seq START 1;
CREATE SEQUENCE IF NOT EXISTS bid_seq START 1;
CREATE SEQUENCE IF NOT EXISTS message_seq START 1;
CREATE SEQUENCE IF NOT EXISTS dispute_seq START 1;
CREATE SEQUENCE IF NOT EXISTS user_seq START 1;
CREATE SEQUENCE IF NOT EXISTS organization_seq START 1;
CREATE SEQUENCE IF NOT EXISTS iam_session_seq START 1;

CREATE TABLE IF NOT EXISTS users (
  id TEXT PRIMARY KEY,
  email TEXT NOT NULL UNIQUE,
  name TEXT NOT NULL,
  password_hash TEXT NOT NULL,
  created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE TABLE IF NOT EXISTS organizations (
  id TEXT PRIMARY KEY,
  name TEXT NOT NULL,
  kind TEXT NOT NULL,
  created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE TABLE IF NOT EXISTS memberships (
  user_id TEXT NOT NULL REFERENCES users(id),
  organization_id TEXT NOT NULL REFERENCES organizations(id),
  role TEXT NOT NULL,
  created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  PRIMARY KEY (user_id, organization_id)
);

CREATE TABLE IF NOT EXISTS iam_sessions (
  id TEXT PRIMARY KEY,
  user_id TEXT NOT NULL REFERENCES users(id),
  token_digest TEXT NOT NULL UNIQUE,
  expires_at TIMESTAMPTZ NOT NULL,
  created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  revoked_at TIMESTAMPTZ
);

ALTER TABLE IF EXISTS iam_sessions
  ADD COLUMN IF NOT EXISTS revoked_at TIMESTAMPTZ;

CREATE TABLE IF NOT EXISTS providers (
  id TEXT PRIMARY KEY,
  name TEXT NOT NULL,
  capabilities JSONB NOT NULL,
  reputation_tier TEXT NOT NULL,
  created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE TABLE IF NOT EXISTS listings (
  id TEXT PRIMARY KEY,
  provider_org_id TEXT NOT NULL REFERENCES providers(id),
  title TEXT NOT NULL,
  category TEXT NOT NULL,
  base_price_cents BIGINT NOT NULL,
  tags JSONB NOT NULL,
  created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE TABLE IF NOT EXISTS orders (
  id TEXT PRIMARY KEY,
  buyer_org_id TEXT NOT NULL,
  provider_org_id TEXT NOT NULL,
  funding_mode TEXT NOT NULL,
  status TEXT NOT NULL,
  payload JSONB NOT NULL,
  created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE TABLE IF NOT EXISTS rfqs (
  id TEXT PRIMARY KEY,
  buyer_org_id TEXT NOT NULL,
  title TEXT NOT NULL,
  category TEXT NOT NULL,
  scope TEXT NOT NULL,
  budget_cents BIGINT NOT NULL,
  default_milestones JSONB NOT NULL DEFAULT '[]'::jsonb,
  status TEXT NOT NULL,
  awarded_bid_id TEXT,
  awarded_provider_org_id TEXT,
  order_id TEXT,
  response_deadline_at TIMESTAMPTZ NOT NULL,
  created_at TIMESTAMPTZ NOT NULL,
  updated_at TIMESTAMPTZ NOT NULL
);

ALTER TABLE IF EXISTS rfqs
  ADD COLUMN IF NOT EXISTS awarded_bid_id TEXT;

ALTER TABLE IF EXISTS rfqs
  ADD COLUMN IF NOT EXISTS awarded_provider_org_id TEXT;

ALTER TABLE IF EXISTS rfqs
  ADD COLUMN IF NOT EXISTS order_id TEXT;

ALTER TABLE IF EXISTS rfqs
  ADD COLUMN IF NOT EXISTS default_milestones JSONB NOT NULL DEFAULT '[]'::jsonb;

CREATE TABLE IF NOT EXISTS bids (
  id TEXT PRIMARY KEY,
  rfq_id TEXT NOT NULL REFERENCES rfqs(id),
  provider_org_id TEXT NOT NULL,
  message TEXT NOT NULL,
  quote_cents BIGINT NOT NULL,
  status TEXT NOT NULL,
  milestones JSONB NOT NULL,
  created_at TIMESTAMPTZ NOT NULL,
  updated_at TIMESTAMPTZ NOT NULL
);

CREATE TABLE IF NOT EXISTS messages (
  id TEXT PRIMARY KEY,
  order_id TEXT,
  rfq_id TEXT,
  author TEXT NOT NULL,
  body TEXT NOT NULL,
  created_at TIMESTAMPTZ NOT NULL
);

CREATE TABLE IF NOT EXISTS disputes (
  id TEXT PRIMARY KEY,
  order_id TEXT NOT NULL,
  milestone_id TEXT NOT NULL,
  reason TEXT NOT NULL,
  refund_cents BIGINT NOT NULL,
  status TEXT NOT NULL DEFAULT 'open',
  resolution TEXT NOT NULL DEFAULT '',
  resolved_by TEXT NOT NULL DEFAULT '',
  resolved_at TIMESTAMPTZ,
  created_at TIMESTAMPTZ NOT NULL
);

ALTER TABLE disputes
  ADD COLUMN IF NOT EXISTS status TEXT NOT NULL DEFAULT 'open';

ALTER TABLE disputes
  ADD COLUMN IF NOT EXISTS resolution TEXT NOT NULL DEFAULT '';

ALTER TABLE disputes
  ADD COLUMN IF NOT EXISTS resolved_by TEXT NOT NULL DEFAULT '';

ALTER TABLE disputes
  ADD COLUMN IF NOT EXISTS resolved_at TIMESTAMPTZ;

-- Indexes on foreign keys and common filter columns (#65)
CREATE INDEX IF NOT EXISTS idx_memberships_org_id ON memberships (organization_id);
CREATE INDEX IF NOT EXISTS idx_iam_sessions_user_id ON iam_sessions (user_id);
CREATE INDEX IF NOT EXISTS idx_iam_sessions_expires_at ON iam_sessions (expires_at) WHERE revoked_at IS NULL;
CREATE INDEX IF NOT EXISTS idx_listings_provider_org_id ON listings (provider_org_id);
CREATE INDEX IF NOT EXISTS idx_orders_buyer_org_id ON orders (buyer_org_id);
CREATE INDEX IF NOT EXISTS idx_orders_provider_org_id ON orders (provider_org_id);
CREATE INDEX IF NOT EXISTS idx_orders_status ON orders (status);
CREATE INDEX IF NOT EXISTS idx_rfqs_buyer_org_id ON rfqs (buyer_org_id);
CREATE INDEX IF NOT EXISTS idx_rfqs_status ON rfqs (status);
CREATE INDEX IF NOT EXISTS idx_bids_rfq_id ON bids (rfq_id);
CREATE INDEX IF NOT EXISTS idx_bids_provider_org_id ON bids (provider_org_id);
CREATE INDEX IF NOT EXISTS idx_messages_order_id ON messages (order_id);
CREATE INDEX IF NOT EXISTS idx_disputes_order_id ON disputes (order_id);
CREATE INDEX IF NOT EXISTS idx_disputes_status ON disputes (status);

ALTER TABLE IF EXISTS messages
  ADD COLUMN IF NOT EXISTS rfq_id TEXT;

ALTER TABLE IF EXISTS messages
  ALTER COLUMN order_id DROP NOT NULL;

