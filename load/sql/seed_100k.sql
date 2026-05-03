-- 1000 chats * 100 links = 100000 tracked links
INSERT INTO chats (id)
SELECT i FROM generate_series(1, 1000) AS g(i)
ON CONFLICT DO NOTHING;

INSERT INTO links (url)
SELECT format('https://example.com/%s/%s', c.i, l.j)
FROM generate_series(1, 1000) AS c(i)
CROSS JOIN generate_series(1, 100) AS l(j)
ON CONFLICT DO NOTHING;

INSERT INTO chat_links (chat_id, link_id)
SELECT c.i, li.id
FROM generate_series(1, 1000) AS c(i)
JOIN links li ON li.url LIKE format('https://example.com/%s/%%', c.i)
ON CONFLICT DO NOTHING;
