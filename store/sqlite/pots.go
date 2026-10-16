package sqlite

import (
	"context"
	"database/sql"
	"errors"
	"strings"
	"time"

	"github.com/vlastrog/greenh/core"
)

var _ core.Store = (*Store)(nil)

const potColumns = `id, name, species, category, pot_ml, soil_type, placement, node_id, channel,
dry_raw, wet_raw, threshold_pct, amount_fraction, check_interval_s, created_at`

type scanner interface{ Scan(dest ...any) error }

func scanPot(sc scanner) (core.Pot, error) {
	var (
		p        core.Pot
		category string
		node     sql.NullString
		ch       sql.NullInt64
		dry, wet sql.NullInt64
		interval int64
		created  int64
	)
	err := sc.Scan(&p.ID, &p.Name, &p.Species, &category, &p.PotML, &p.SoilType, &p.Placement,
		&node, &ch, &dry, &wet, &p.Rules.ThresholdPct, &p.Rules.AmountFraction, &interval, &created)
	if err != nil {
		return core.Pot{}, err
	}
	p.Category = core.Category(category)
	if node.Valid {
		p.NodeID = &node.String
	}
	p.Channel = nullInt(ch)
	p.DryRaw, p.WetRaw = nullInt(dry), nullInt(wet)
	p.CheckInterval = time.Duration(interval) * time.Second
	p.CreatedAt = fromMillis(created)
	return p, nil
}

func nullInt(n sql.NullInt64) *int {
	if !n.Valid {
		return nil
	}
	v := int(n.Int64)
	return &v
}

// mapConflict turns a unique-constraint violation into core.ErrConflict.
func mapConflict(err error) error {
	if err != nil && strings.Contains(err.Error(), "UNIQUE constraint failed") {
		return core.ErrConflict
	}
	return err
}

func (s *Store) CreatePot(ctx context.Context, p core.Pot) (core.Pot, error) {
	res, err := s.db.ExecContext(ctx, `
INSERT INTO pots (name, species, category, pot_ml, soil_type, placement, node_id, channel,
  dry_raw, wet_raw, threshold_pct, amount_fraction, check_interval_s, created_at)
VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		p.Name, p.Species, string(p.Category), p.PotML, p.SoilType, p.Placement, p.NodeID, p.Channel,
		p.DryRaw, p.WetRaw, p.Rules.ThresholdPct, p.Rules.AmountFraction,
		int64(p.CheckInterval/time.Second), p.CreatedAt.UnixMilli())
	if err != nil {
		return core.Pot{}, mapConflict(err)
	}
	id, err := res.LastInsertId()
	if err != nil {
		return core.Pot{}, err
	}
	return s.GetPot(ctx, id)
}

func (s *Store) GetPot(ctx context.Context, id int64) (core.Pot, error) {
	p, err := scanPot(s.db.QueryRowContext(ctx, `SELECT `+potColumns+` FROM pots WHERE id = ?`, id))
	if errors.Is(err, sql.ErrNoRows) {
		return core.Pot{}, core.ErrNotFound
	}
	return p, err
}

func (s *Store) ListPots(ctx context.Context) ([]core.Pot, error) {
	rows, err := s.db.QueryContext(ctx, `SELECT `+potColumns+` FROM pots ORDER BY id`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var pots []core.Pot
	for rows.Next() {
		p, err := scanPot(rows)
		if err != nil {
			return nil, err
		}
		pots = append(pots, p)
	}
	return pots, rows.Err()
}

// UpdatePot replaces every field except created_at.
func (s *Store) UpdatePot(ctx context.Context, p core.Pot) (core.Pot, error) {
	res, err := s.db.ExecContext(ctx, `
UPDATE pots SET name = ?, species = ?, category = ?, pot_ml = ?, soil_type = ?, placement = ?,
  node_id = ?, channel = ?, dry_raw = ?, wet_raw = ?, threshold_pct = ?, amount_fraction = ?,
  check_interval_s = ?
WHERE id = ?`,
		p.Name, p.Species, string(p.Category), p.PotML, p.SoilType, p.Placement, p.NodeID, p.Channel,
		p.DryRaw, p.WetRaw, p.Rules.ThresholdPct, p.Rules.AmountFraction,
		int64(p.CheckInterval/time.Second), p.ID)
	if err != nil {
		return core.Pot{}, mapConflict(err)
	}
	n, err := res.RowsAffected()
	if err != nil {
		return core.Pot{}, err
	}
	if n == 0 {
		return core.Pot{}, core.ErrNotFound
	}
	return s.GetPot(ctx, p.ID)
}

func (s *Store) DeletePot(ctx context.Context, id int64) error {
	res, err := s.db.ExecContext(ctx, `DELETE FROM pots WHERE id = ?`, id)
	if err != nil {
		return err
	}
	n, err := res.RowsAffected()
	if err != nil {
		return err
	}
	if n == 0 {
		return core.ErrNotFound
	}
	return nil
}
