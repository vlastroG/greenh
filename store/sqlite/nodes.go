package sqlite

import (
	"context"
	"database/sql"
	"time"

	"github.com/vlastrog/greenh/core"
)

const nodeChannelSelect = `
SELECT nc.node_id, nc.ch,
  (SELECT moisture_raw FROM channel_readings WHERE node_id = nc.node_id AND ch = nc.ch ORDER BY ts DESC LIMIT 1),
  (SELECT ts FROM channel_readings WHERE node_id = nc.node_id AND ch = nc.ch ORDER BY ts DESC LIMIT 1),
  (SELECT id FROM pots WHERE node_id = nc.node_id AND channel = nc.ch)
FROM node_channels nc
ORDER BY nc.node_id, nc.ch`

func (s *Store) ListNodes(ctx context.Context) ([]core.Node, error) {
	return s.queryNodes(ctx, "")
}

func (s *Store) RenameNode(ctx context.Context, id, name string) (core.Node, error) {
	res, err := s.db.ExecContext(ctx, `UPDATE nodes SET name = ? WHERE id = ?`, name, id)
	if err != nil {
		return core.Node{}, err
	}
	n, err := res.RowsAffected()
	if err != nil {
		return core.Node{}, err
	}
	if n == 0 {
		return core.Node{}, core.ErrNotFound
	}
	nodes, err := s.queryNodes(ctx, id)
	if err != nil {
		return core.Node{}, err
	}
	if len(nodes) == 0 {
		return core.Node{}, core.ErrNotFound
	}
	return nodes[0], nil
}

// queryNodes loads nodes with their channels; onlyID restricts it to one node when set.
func (s *Store) queryNodes(ctx context.Context, onlyID string) ([]core.Node, error) {
	query, args := `SELECT id, name, last_seen FROM nodes ORDER BY id`, []any(nil)
	if onlyID != "" {
		query, args = `SELECT id, name, last_seen FROM nodes WHERE id = ?`, []any{onlyID}
	}
	rows, err := s.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	var nodes []core.Node
	byID := map[string]int{}
	for rows.Next() {
		var n core.Node
		var seen int64
		if err := rows.Scan(&n.ID, &n.Name, &seen); err != nil {
			rows.Close()
			return nil, err
		}
		n.LastSeen = fromMillis(seen)
		n.Channels = []core.NodeChannel{}
		byID[n.ID] = len(nodes)
		nodes = append(nodes, n)
	}
	rows.Close()
	if err := rows.Err(); err != nil {
		return nil, err
	}

	// the pool has one connection, so the node rows must be closed before this query
	crows, err := s.db.QueryContext(ctx, nodeChannelSelect)
	if err != nil {
		return nil, err
	}
	defer crows.Close()
	for crows.Next() {
		var (
			nodeID string
			c      core.NodeChannel
			raw    sql.NullInt64
			at     sql.NullInt64
			pot    sql.NullInt64
		)
		if err := crows.Scan(&nodeID, &c.Ch, &raw, &at, &pot); err != nil {
			return nil, err
		}
		i, ok := byID[nodeID]
		if !ok {
			continue
		}
		if raw.Valid {
			v := int(raw.Int64)
			c.LastRaw = &v
		}
		if at.Valid {
			t := time.UnixMilli(at.Int64).UTC()
			c.LastAt = &t
		}
		if pot.Valid {
			c.PotID = &pot.Int64
		}
		nodes[i].Channels = append(nodes[i].Channels, c)
	}
	return nodes, crows.Err()
}
