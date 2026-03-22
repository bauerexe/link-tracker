CREATE INDEX idx_chat_links_chat_id ON chat_links (chat_id);
CREATE INDEX idx_chat_links_link_id ON chat_links (link_id);
CREATE INDEX idx_links_last_checked_at ON links (last_checked_at);
CREATE INDEX idx_chat_link_tags_chat_link_id ON chat_link_tags (chat_link_id);
CREATE INDEX idx_chat_link_tags_tag_id ON chat_link_tags (tag_id);
CREATE INDEX idx_chat_link_filters_chat_link_id ON chat_link_filters (chat_link_id);