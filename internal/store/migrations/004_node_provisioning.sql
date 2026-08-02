CREATE TABLE media_node_provisioning (
    node_id BIGINT PRIMARY KEY REFERENCES media_nodes(id) ON DELETE CASCADE,
    wireguard_address INET NOT NULL UNIQUE,
    wireguard_public_key TEXT NOT NULL UNIQUE,
    wireguard_private_key_sealed TEXT NOT NULL,
    api_token_sealed TEXT NOT NULL,
    relay_token_sealed TEXT NOT NULL,
    bundle_downloaded_at TIMESTAMPTZ,
    revoked_at TIMESTAMPTZ,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);
