package sqlite

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"

	"github.com/vlastrog/greenh/core"
)

var (
	_ core.ReadingStore = (*Store)(nil)
	_ core.NodeStore    = (*Store)(nil)
)

func fromMillis(ms int64) time.Time { return time.UnixMilli(ms).UTC() }

func nullFloat(p *float64) sql.NullFloat64 {
	if p == nil {
		return sql.NullFloat64{}
	}
	return sql.NullFloat64{Float64: *p, Valid: true}
}

func floatPtr(n sql.NullFloat64) *float64 {
	if !n.Valid {
		return nil
	}
	v := n.Float64
	return &v
}

func (s *Store) InsertReading(ctx context.Context, r core.Reading) (bool, error) {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return false, err
	}
	defer tx.Rollback()

	ts := r.TS.UnixMilli()
	res, err := tx.ExecContext(ctx,
		`INSERT OR IGNORE INTO readings (node_id, ts, temp_c, humidity_pct, light_lux) VALUES (?, ?, ?, ?, ?)`,
		r.NodeID, ts, nullFloat(r.Ambient.TempC), nullFloat(r.Ambient.HumidityPct), nullFloat(r.Ambient.LightLux))
	if err != nil {
		return false, fmt.Errorf("insert reading: %w", err)
	}
	n, err := res.RowsAffected()
	if err != nil {
		return false, err
	}
	if n == 0 {
		return false, nil
	}

	if _, err := tx.ExecContext(ctx,
		`INSERT INTO nodes (id, last_seen) VALUES (?, ?)
		 ON CONFLICT(id) DO UPDATE SET last_seen = MAX(last_seen, excluded.last_seen)`,
		r.NodeID, ts); err != nil {
		return false, fmt.Errorf("upsert node: %w", err)
	}
	for _, c := range r.Channels {
		if _, err := tx.ExecContext(ctx,
			`INSERT OR IGNORE INTO node_channels (node_id, ch) VALUES (?, ?)`, r.NodeID, c.Ch); err != nil {
			return false, fmt.Errorf("upsert channel: %w", err)
		}
		if _, err := tx.ExecContext(ctx,
			`INSERT INTO channel_readings (node_id, ts, ch, moisture_raw) VALUES (?, ?, ?, ?)`,
			r.NodeID, ts, c.Ch, c.MoistureRaw); err != nil {
			return false, fmt.Errorf("insert channel reading: %w", err)
		}
	}
	if err := tx.Commit(); err != nil {
		return false, err
	}
	return true, nil
}

const pointSelect = `
SELECT cr.ts, cr.moisture_raw, r.temp_c, r.humidity_pct, r.light_lux
FROM channel_readings cr
JOIN readings r ON r.node_id = cr.node_id AND r.ts = cr.ts
WHERE cr.node_id = ? AND cr.ch = ?`

type rowScanner interface{ Scan(dest ...any) error }

func scanPoint(row rowScanner) (core.RawPoint, error) {
	var (
		ts, raw       int64
		temp, hum, lx sql.NullFloat64
	)
	if err := row.Scan(&ts, &raw, &temp, &hum, &lx); err != nil {
		return core.RawPoint{}, err
	}
	return core.RawPoint{
		TS:          fromMillis(ts),
		MoistureRaw: int(raw),
		Ambient:     core.Ambient{TempC: floatPtr(temp), HumidityPct: floatPtr(hum), LightLux: floatPtr(lx)},
	}, nil
}

func (s *Store) History(ctx context.Context, nodeID string, ch int, from, to time.Time) ([]core.RawPoint, error) {
	rows, err := s.db.QueryContext(ctx,
		pointSelect+` AND cr.ts >= ? AND cr.ts < ? ORDER BY cr.ts ASC`,
		nodeID, ch, from.UnixMilli(), to.UnixMilli())
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []core.RawPoint
	for rows.Next() {
		p, err := scanPoint(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, p)
	}
	return out, rows.Err()
}

func (s *Store) LastRaw(ctx context.Context, nodeID string, ch int) (*core.RawPoint, error) {
	p, err := scanPoint(s.db.QueryRowContext(ctx, pointSelect+` ORDER BY cr.ts DESC LIMIT 1`, nodeID, ch))
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &p, nil
}
