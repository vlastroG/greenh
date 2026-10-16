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

type PotStore interface {
	// CreatePot returns ErrConflict if (node_id, channel) is already linked.
	CreatePot(ctx context.Context, p Pot) (Pot, error)
	GetPot(ctx context.Context, id int64) (Pot, error)
	// ListPots returns pots ordered by id.
	ListPots(ctx context.Context) ([]Pot, error)
	// UpdatePot fully replaces the pot by p.ID; ErrNotFound or ErrConflict on failure.
	UpdatePot(ctx context.Context, p Pot) (Pot, error)
	// DeletePot cascades to the pot's waterings and advice.
	DeletePot(ctx context.Context, id int64) error
}

type WateringStore interface {
	CreateWatering(ctx context.Context, w Watering) (Watering, error)
	// ListWaterings returns the newest first.
	ListWaterings(ctx context.Context, potID int64, limit int) ([]Watering, error)
	// LastWatering returns the watering with the newest TS, or nil if none.
	LastWatering(ctx context.Context, potID int64) (*Watering, error)
	// WateringsSince returns waterings with created_at > since, ascending by created_at.
	WateringsSince(ctx context.Context, since time.Time) ([]WateringLink, error)
}

type AdviceStore interface {
	InsertAdvice(ctx context.Context, a Advice) (Advice, error)
	// ListAdvice returns the newest first.
	ListAdvice(ctx context.Context, potID int64, limit int) ([]Advice, error)
	// LatestAdvice returns nil if the pot has none.
	LatestAdvice(ctx context.Context, potID int64) (*Advice, error)
	// LastAdviceTimes maps pot id to its newest advice created_at.
	LastAdviceTimes(ctx context.Context) (map[int64]time.Time, error)
}

type Store interface {
	ReadingStore
	NodeStore
	PotStore
	WateringStore
	AdviceStore
}
