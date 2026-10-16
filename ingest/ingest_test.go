package ingest_test

import (
	"context"
	"errors"
	"io"
	"log/slog"
	"strings"
	"testing"
	"time"

	"github.com/vlastrog/greenh/core"
	"github.com/vlastrog/greenh/ingest"
	"github.com/vlastrog/greenh/store/sqlite/sqlitetest"
)

var now = time.Date(2026, 10, 10, 12, 0, 0, 0, time.UTC)

func fp(v float64) *float64 { return &v }

func newService(t *testing.T) (*ingest.Service, core.ReadingStore) {
	t.Helper()
	st := sqlitetest.New(t)
	log := slog.New(slog.NewTextHandler(io.Discard, nil))
	return ingest.New(st, func() time.Time { return now }, log), st
}

func valid() core.Reading {
	return core.Reading{
		NodeID:   "node-1",
		TS:       now,
		Channels: []core.ChannelReading{{Ch: 0, MoistureRaw: 2000}, {Ch: 1, MoistureRaw: 3000}},
	}
}

func TestIngestStoresWithReceiveTimeWhenTSMissing(t *testing.T) {
	svc, st := newService(t)
	r := valid()
	r.TS = time.Time{}
	stored, err := svc.Ingest(context.Background(), r)
	if err != nil || !stored {
		t.Fatalf("stored=%v err=%v", stored, err)
	}
	p, err := st.LastRaw(context.Background(), "node-1", 0)
	if err != nil || p == nil {
		t.Fatalf("last raw: %v %v", p, err)
	}
	if !p.TS.Equal(now) {
		t.Errorf("ts = %v, want %v", p.TS, now)
	}
}

func TestIngestRejects(t *testing.T) {
	tests := []struct {
		name  string
		mod   func(r *core.Reading)
		field string
	}{
		{"empty node_id", func(r *core.Reading) { r.NodeID = "" }, "node_id"},
		{"bad node_id", func(r *core.Reading) { r.NodeID = "a b" }, "node_id"},
		{"long node_id", func(r *core.Reading) { r.NodeID = strings.Repeat("a", 65) }, "node_id"},
		{"ts too old", func(r *core.Reading) { r.TS = now.Add(-8 * 24 * time.Hour) }, "ts"},
		{"ts in future", func(r *core.Reading) { r.TS = now.Add(6 * time.Minute) }, "ts"},
		{"no channels", func(r *core.Reading) { r.Channels = nil }, "channels"},
		{"ch too high", func(r *core.Reading) { r.Channels[1].Ch = 16 }, "channels[1].ch"},
		{"duplicate ch", func(r *core.Reading) { r.Channels[1].Ch = 0 }, "channels[1].ch"},
		{"raw too high", func(r *core.Reading) { r.Channels[1].MoistureRaw = 4096 }, "channels[1].moisture_raw"},
		{"raw negative", func(r *core.Reading) { r.Channels[1].MoistureRaw = -1 }, "channels[1].moisture_raw"},
		{"temp", func(r *core.Reading) { r.Ambient.TempC = fp(86) }, "ambient.temp_c"},
		{"humidity", func(r *core.Reading) { r.Ambient.HumidityPct = fp(101) }, "ambient.humidity_pct"},
		{"light", func(r *core.Reading) { r.Ambient.LightLux = fp(200001) }, "ambient.light_lux"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			svc, _ := newService(t)
			r := valid()
			tc.mod(&r)
			stored, err := svc.Ingest(context.Background(), r)
			var verr *core.ValidationError
			if !errors.As(err, &verr) {
				t.Fatalf("err = %v, want *core.ValidationError", err)
			}
			if stored {
				t.Error("stored = true on rejection")
			}
			found := false
			for _, f := range verr.Fields {
				found = found || f.Field == tc.field
			}
			if !found {
				t.Errorf("fields = %+v, want %q", verr.Fields, tc.field)
			}
		})
	}
}

func TestIngestCollectsAllFieldErrors(t *testing.T) {
	svc, _ := newService(t)
	r := valid()
	r.NodeID = ""
	r.Channels[0].MoistureRaw = 5000
	_, err := svc.Ingest(context.Background(), r)
	var verr *core.ValidationError
	if !errors.As(err, &verr) || len(verr.Fields) != 2 {
		t.Fatalf("err = %v", err)
	}
}

func TestIngestAcceptsBoundaries(t *testing.T) {
	for _, ts := range []time.Time{now.Add(-7*24*time.Hour + time.Second), now.Add(5 * time.Minute)} {
		svc, _ := newService(t)
		r := valid()
		r.TS = ts
		r.Channels = []core.ChannelReading{{Ch: 0, MoistureRaw: 0}, {Ch: 15, MoistureRaw: 4095}}
		if stored, err := svc.Ingest(context.Background(), r); err != nil || !stored {
			t.Errorf("ts %v: stored=%v err=%v", ts, stored, err)
		}
	}
}

func TestIngestDuplicateReturnsNotStored(t *testing.T) {
	svc, _ := newService(t)
	ctx := context.Background()
	if stored, err := svc.Ingest(ctx, valid()); err != nil || !stored {
		t.Fatalf("first: stored=%v err=%v", stored, err)
	}
	if stored, err := svc.Ingest(ctx, valid()); err != nil || stored {
		t.Fatalf("second: stored=%v err=%v", stored, err)
	}
}
