package observability

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/prometheus/client_golang/prometheus"
)

const trackedLinksBySourceSQL = `
SELECT
	CASE
		WHEN lower(l.url) LIKE '%github.com/%' OR lower(l.url) LIKE '%github.com' THEN 'github'
		WHEN lower(l.url) LIKE '%stackoverflow.com/%' OR lower(l.url) LIKE '%stackoverflow.com' THEN 'stackoverflow'
		ELSE 'other'
	END AS tracked_source,
	COUNT(DISTINCT l.id)::double precision AS links_count
FROM links l
JOIN chat_links cl ON cl.link_id = l.id
GROUP BY tracked_source
`

var (
	registerTrackedLinksOnce sync.Once
	registerTrackedLinksErr  error
)

type TrackedLinksCollector struct {
	pool *pgxpool.Pool
	desc *prometheus.Desc
}

func RegisterTrackedLinksCollector(pool *pgxpool.Pool) error {
	if pool == nil {
		return nil
	}

	registerTrackedLinksOnce.Do(func() {
		err := prometheus.Register(NewTrackedLinksCollector(pool))
		if alreadyRegistered, ok := err.(prometheus.AlreadyRegisteredError); ok {
			if _, ok = alreadyRegistered.ExistingCollector.(*TrackedLinksCollector); ok {
				err = nil
			}
		}
		registerTrackedLinksErr = err
	})

	return registerTrackedLinksErr
}

func NewTrackedLinksCollector(pool *pgxpool.Pool) *TrackedLinksCollector {
	return &TrackedLinksCollector{
		pool: pool,
		desc: prometheus.NewDesc(
			"links_on_track_total",
			"Current number of active tracked links grouped by normalized source.",
			[]string{"tracked_source"},
			nil,
		),
	}
}

func (c *TrackedLinksCollector) Describe(ch chan<- *prometheus.Desc) {
	ch <- c.desc
}

func (c *TrackedLinksCollector) Collect(ch chan<- prometheus.Metric) {
	started := time.Now()
	defer ObserveRequestDuration(ScopeDatabase, "links", started)

	counts, err := c.collectCounts()
	if err != nil {
		ch <- prometheus.NewInvalidMetric(c.desc, err)
		return
	}

	for _, source := range []string{"github", "stackoverflow", "other"} {
		ch <- prometheus.MustNewConstMetric(
			c.desc,
			prometheus.GaugeValue,
			counts[source],
			source,
		)
	}
}

func (c *TrackedLinksCollector) collectCounts() (map[string]float64, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	rows, err := c.pool.Query(ctx, trackedLinksBySourceSQL)
	if err != nil {
		return nil, fmt.Errorf("collect tracked links: %w", err)
	}
	defer rows.Close()

	counts := map[string]float64{
		"github":        0,
		"stackoverflow": 0,
		"other":         0,
	}

	for rows.Next() {
		var (
			source string
			count  float64
		)
		if err = rows.Scan(&source, &count); err != nil {
			return nil, fmt.Errorf("scan tracked links count: %w", err)
		}
		counts[source] = count
	}

	if err = rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate tracked links count: %w", err)
	}

	return counts, nil
}
