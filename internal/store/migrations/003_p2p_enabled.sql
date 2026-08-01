INSERT INTO site_settings(key,value) VALUES('p2p_enabled','false') ON CONFLICT(key) DO NOTHING;
DELETE FROM site_settings WHERE key='user_stream_bps';
