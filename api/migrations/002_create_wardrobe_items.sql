CREATE TABLE wardrobe_items (
    id BIGSERIAL PRIMARY KEY,
    user_id BIGINT NOT NULL REFERENCES users(id),
    name TEXT NOT NULL,
    category TEXT NOT NULL,
    color TEXT NOT NULL,
    image_url TEXT NOT NULL DEFAULT '',
    source TEXT NOT NULL DEFAULT 'Manual',
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);
