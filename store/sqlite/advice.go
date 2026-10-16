package sqlite

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"

	"github.com/vlastrog/greenh/core"
)

const adviceColumns = `id, pot_id, created_at, trigger, status, recommendation, rules, provider, model,
prompt_version, fallback, fallback_reason, disagree, summary`

// jsonOrNull encodes v as JSON text, or NULL when v is nil.
func jsonOrNull(v *core.Recommendation) (any, error) {
	if v == nil {
		return nil, nil
	}
	b, err := json.Marshal(v)
	if err != nil {
		return nil, err
	}
	return string(b), nil
}

func decodeRecommendation(s sql.NullString) (*core.Recommendation, error) {
	if !s.Valid {
		return nil, nil
	}
	var r core.Recommendation
	if err := json.Unmarshal([]byte(s.String), &r); err != nil {
		return nil, err
	}
	return &r, nil
}

func scanAdvice(sc scanner) (core.Advice, error) {
	var (
		a                     core.Advice
		created               int64
		trigger, status       string
		recommendation, rules sql.NullString
		summary               sql.NullString
	)
	err := sc.Scan(&a.ID, &a.PotID, &created, &trigger, &status, &recommendation, &rules,
		&a.Provider, &a.Model, &a.PromptVersion, &a.Fallback, &a.FallbackReason, &a.Disagree, &summary)
	if err != nil {
		return core.Advice{}, err
	}
	a.CreatedAt = fromMillis(created)
	a.Trigger, a.Status = core.Trigger(trigger), core.Status(status)
	if a.Recommendation, err = decodeRecommendation(recommendation); err != nil {
		return core.Advice{}, err
	}
	if a.Rules, err = decodeRecommendation(rules); err != nil {
		return core.Advice{}, err
	}
	if summary.Valid {
		a.Summary = json.RawMessage(summary.String)
	}
	return a, nil
}

func (s *Store) InsertAdvice(ctx context.Context, a core.Advice) (core.Advice, error) {
	rec, err := jsonOrNull(a.Recommendation)
	if err != nil {
		return core.Advice{}, err
	}
	rules, err := jsonOrNull(a.Rules)
	if err != nil {
		return core.Advice{}, err
	}
	var summary any
	if len(a.Summary) > 0 {
		summary = string(a.Summary)
	}
	res, err := s.db.ExecContext(ctx, `
INSERT INTO advice (pot_id, created_at, trigger, status, recommendation, rules, provider, model,
  prompt_version, fallback, fallback_reason, disagree, summary)
VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		a.PotID, a.CreatedAt.UnixMilli(), string(a.Trigger), string(a.Status), rec, rules,
		a.Provider, a.Model, a.PromptVersion, a.Fallback, a.FallbackReason, a.Disagree, summary)
	if err != nil {
		return core.Advice{}, err
	}
	a.ID, err = res.LastInsertId()
	return a, err
}

func (s *Store) ListAdvice(ctx context.Context, potID int64, limit int) ([]core.Advice, error) {
	rows, err := s.db.QueryContext(ctx, `SELECT `+adviceColumns+
		` FROM advice WHERE pot_id = ? ORDER BY created_at DESC, id DESC LIMIT ?`, potID, sqlLimit(limit))
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []core.Advice
	for rows.Next() {
		a, err := scanAdvice(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, a)
	}
	return out, rows.Err()
}

func (s *Store) LatestAdvice(ctx context.Context, potID int64) (*core.Advice, error) {
	a, err := scanAdvice(s.db.QueryRowContext(ctx, `SELECT `+adviceColumns+
		` FROM advice WHERE pot_id = ? ORDER BY created_at DESC, id DESC LIMIT 1`, potID))
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &a, nil
}

func (s *Store) LastAdviceStamps(ctx context.Context) (map[int64]core.AdviceStamp, error) {
	rows, err := s.db.QueryContext(ctx, `SELECT a.pot_id, a.created_at, a.status FROM advice a
WHERE a.id = (SELECT b.id FROM advice b WHERE b.pot_id = a.pot_id ORDER BY b.created_at DESC, b.id DESC LIMIT 1)`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := map[int64]core.AdviceStamp{}
	for rows.Next() {
		var id, at int64
		var status string
		if err := rows.Scan(&id, &at, &status); err != nil {
			return nil, err
		}
		out[id] = core.AdviceStamp{At: fromMillis(at), Status: core.Status(status)}
	}
	return out, rows.Err()
}
