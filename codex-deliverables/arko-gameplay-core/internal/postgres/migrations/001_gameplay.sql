CREATE TABLE IF NOT EXISTS arko_gameplay_items (
	item_id TEXT PRIMARY KEY,
	name TEXT NOT NULL,
	kind TEXT NOT NULL CHECK (kind IN ('material', 'equipment', 'generic')),
	max_stack BIGINT NOT NULL CHECK (max_stack > 0)
);

CREATE TABLE IF NOT EXISTS arko_gameplay_recipes (
	recipe_id TEXT NOT NULL,
	recipe_version BIGINT NOT NULL CHECK (recipe_version > 0),
	name TEXT NOT NULL,
	materials JSONB NOT NULL,
	product JSONB NOT NULL,
	requirements JSONB NOT NULL,
	active BOOLEAN NOT NULL DEFAULT FALSE,
	demo_only BOOLEAN NOT NULL DEFAULT FALSE,
	PRIMARY KEY (recipe_id, recipe_version)
);

CREATE TABLE IF NOT EXISTS arko_gameplay_inventories (
	character_id TEXT PRIMARY KEY,
	slots JSONB NOT NULL
);

CREATE TABLE IF NOT EXISTS arko_gameplay_operations (
	character_id TEXT NOT NULL,
	operation_type TEXT NOT NULL,
	idempotency_key TEXT NOT NULL,
	request_id TEXT NOT NULL,
	fingerprint TEXT NOT NULL,
	operation_id TEXT NOT NULL,
	result JSONB NOT NULL,
	created_at TIMESTAMPTZ NOT NULL,
	updated_at TIMESTAMPTZ NOT NULL,
	PRIMARY KEY (character_id, operation_type, idempotency_key)
);
