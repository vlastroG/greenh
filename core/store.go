package core

import (
	"context"
	"time"
)

type ReadingStore interface {
	// InsertReading returns false on a duplicate (node_id, ts). It also registers the
	// node and its channels and advances node.last_seen to max(last_seen, ts).
	InsertReading(ctx context.Context, r Reading) (stored bool, err error)
	// History returns points ascending, inclusive of from and exclusive of to.
	History(ctx context.Context, nodeID string, ch int, from, to time.Time) ([]RawPoint, error)
	// LastRaw returns nil, nil when the channel has no readings.
	LastRaw(ctx context.Context, nodeID string, ch int) (*RawPoint, error)
}

type NodeStore interface {
	ListNodes(ctx context.Context) ([]Node, error)
	RenameNode(ctx context.Context, id, name string) (Node, error)
}
