package sqlite

import (
	"context"
	"database/sql"
	"errors"
	"time"

	"github.com/vlastrog/greenh/core"
)

const wateringColumns = `id, pot_id, ts, created_at, amount_ml, source`

func scanWatering(sc scanner) (core.Watering, error) {
	var w core.Watering
	var ts, created int64
	if err := sc.Scan(&w.ID, &w.PotID, &ts, &created, &w.AmountML, &w.Source); err != nil {
		return core.Watering{}, err
	}
	w.TS, w.CreatedAt = fromMillis(ts), fromMillis(created)
	return w, nil
}

func (s *Store) CreateWatering(ctx context.Context, w core.Watering) (core.Watering, error) {
	res, err := s.db.ExecContext(ctx,
		`INSERT INTO waterings (pot_id, ts, created_at, amount_ml, source) VALUES (?, ?, ?, ?, ?)`,
		w.PotID, w.TS.UnixMilli(), w.CreatedAt.UnixMilli(), w.AmountML, w.Source)
	if err != nil {
		return core.Watering{}, err
	}
	w.ID, err = res.LastInsertId()
	return w, err
}

// sqlLimit maps a non-positive limit to "no limit".
func sqlLimit(limit int) int {
	if limit <= 0 {
		return -1
	}
	return limit
}

func (s *Store) ListWaterings(ctx context.Context, potID int64, limit int) ([]core.Watering, error) {
	rows, err := s.db.QueryContext(ctx, `SELECT `+wateringColumns+
		` FROM waterings WHERE pot_id = ? ORDER BY ts DESC, id DESC LIMIT ?`, potID, sqlLimit(limit))
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []core.Watering
	for rows.Next() {
		w, err := scanWatering(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, w)
	}
	return out, rows.Err()
}

func (s *Store) LastWatering(ctx context.Context, potID int64) (*core.Watering, error) {
	w, err := scanWatering(s.db.QueryRowContext(ctx, `SELECT `+wateringColumns+
		` FROM waterings WHERE pot_id = ? ORDER BY ts DESC, id DESC LIMIT 1`, potID))
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &w, nil
}

func (s *Store) WateringsSince(ctx context.Context, since time.Time) ([]core.WateringLink, error) {
	rows, err := s.db.QueryContext(ctx, `
SELECT w.id, w.pot_id, w.ts, w.created_at, w.amount_ml, w.source, p.node_id, p.channel
FROM waterings w JOIN pots p ON p.id = w.pot_id
WHERE w.created_at > ?
ORDER BY w.created_at, w.id`, since.UnixMilli())
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []core.WateringLink
	for rows.Next() {
		var (
			l           core.WateringLink
			ts, created int64
			node        sql.NullString
			ch          sql.NullInt64
		)
		if err := rows.Scan(&l.ID, &l.PotID, &ts, &created, &l.AmountML, &l.Source, &node, &ch); err != nil {
			return nil, err
		}
		l.TS, l.CreatedAt = fromMillis(ts), fromMillis(created)
		if node.Valid {
			l.NodeID = &node.String
		}
		l.Channel = nullInt(ch)
		out = append(out, l)
	}
	return out, rows.Err()
}
