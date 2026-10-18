// Package monitor wraps a provider with local history: every snapshot is
// recorded in the store, and stored data is used when the cloud is
// unreachable or silent.
package monitor

import (
	"context"
	"fmt"
	"sort"
	"time"

	"github.com/farazmazhar/solar-moonitor-go/internal/provider"
	"github.com/farazmazhar/solar-moonitor-go/internal/store"
)

// Service is a provider.Source backed by another provider and a store.
type Service struct {
	src          provider.Source
	store        *store.Store
	snapshotDays int
}

// New returns a recording service. A nil store disables persistence and the
// service behaves like a pass-through. snapshotDays controls how long raw
// JSON snapshots are kept; zero means 30.
func New(src provider.Source, st *store.Store, snapshotDays int) *Service {
	if snapshotDays <= 0 {
		snapshotDays = 30
	}
	return &Service{src: src, store: st, snapshotDays: snapshotDays}
}

// Name implements provider.Source.
func (s *Service) Name() string {
	if s.src == nil {
		return "none"
	}
	return s.src.Name()
}

// Store exposes the underlying store, which may be nil.
func (s *Service) Store() *store.Store { return s.store }

// Close implements provider.Source.
func (s *Service) Close() error {
	var first error
	if s.src != nil {
		first = s.src.Close()
	}
	if s.store != nil {
		if err := s.store.Close(); err != nil && first == nil {
			first = err
		}
	}
	return first
}

// Snapshot records a live reading, or falls back to the last stored one.
func (s *Service) Snapshot(ctx context.Context) (provider.Snapshot, error) {
	if s.src == nil {
		return provider.Snapshot{}, fmt.Errorf("monitor: no provider")
	}
	snap, err := s.src.Snapshot(ctx)
	if err != nil {
		if s.store != nil {
			if cached, ok, cerr := s.store.LatestSnapshot(ctx); cerr == nil && ok {
				cached.Source = s.src.Name() + " (cached)"
				return cached, nil
			}
		}
		return provider.Snapshot{}, err
	}
	if s.store != nil {
		_ = s.store.SaveSnapshot(ctx, snap)
		_ = s.store.PruneSnapshots(ctx, s.snapshotDays)
	}
	return snap, nil
}

// Today merges the provider's day curve with locally recorded samples.
func (s *Service) Today(ctx context.Context, day time.Time) ([]provider.Sample, error) {
	var remote []provider.Sample
	var remoteErr error
	if s.src != nil {
		remote, remoteErr = s.src.Today(ctx, day)
	}
	local, localErr := s.localSamples(ctx, day)
	if remoteErr != nil && len(local) == 0 {
		return nil, remoteErr
	}
	if localErr != nil && len(remote) == 0 {
		return nil, localErr
	}
	return mergeSamples(remote, local), nil
}

// Month returns daily energy for a month, recording what it fetches.
func (s *Service) Month(ctx context.Context, year int, month time.Month) ([]provider.DayEnergy, error) {
	var remote []provider.DayEnergy
	var remoteErr error
	if s.src != nil {
		remote, remoteErr = s.src.Month(ctx, year, month)
	}
	if s.store != nil {
		if len(remote) > 0 {
			_ = s.store.SaveDayEnergy(ctx, remote)
		}
		local, err := s.store.DayEnergy(ctx, year, month)
		if err == nil && len(local) > 0 {
			if len(remote) == 0 {
				return local, nil
			}
			return remote, nil
		}
		if err != nil && remoteErr != nil {
			return nil, remoteErr
		}
	}
	if remoteErr != nil && len(remote) == 0 {
		return nil, remoteErr
	}
	return remote, nil
}

// Year returns monthly energy for a year, recording what it fetches.
func (s *Service) Year(ctx context.Context, year int) ([]provider.MonthEnergy, error) {
	var remote []provider.MonthEnergy
	var remoteErr error
	if s.src != nil {
		remote, remoteErr = s.src.Year(ctx, year)
	}
	if s.store != nil {
		if len(remote) > 0 {
			_ = s.store.SaveMonthEnergy(ctx, remote)
		}
		local, err := s.store.MonthEnergy(ctx, year)
		if err == nil && len(local) > 0 {
			if len(remote) == 0 {
				return local, nil
			}
			return remote, nil
		}
		if err != nil && remoteErr != nil {
			return nil, remoteErr
		}
	}
	if remoteErr != nil && len(remote) == 0 {
		return nil, remoteErr
	}
	return remote, nil
}

// Alarms returns live alarms, or the stored list when the cloud is down.
func (s *Service) Alarms(ctx context.Context, from, to time.Time) ([]provider.Alarm, error) {
	var remote []provider.Alarm
	var remoteErr error
	if s.src != nil {
		remote, remoteErr = s.src.Alarms(ctx, from, to)
	}
	if s.store != nil {
		if len(remote) > 0 {
			_ = s.store.SaveAlarms(ctx, remote)
		}
		local, err := s.store.Alarms(ctx, from, to)
		if err == nil && len(local) > 0 {
			if len(remote) == 0 {
				return local, nil
			}
			return mergeAlarms(remote, local), nil
		}
	}
	if remoteErr != nil && len(remote) == 0 {
		return nil, remoteErr
	}
	return remote, nil
}

// Seed returns the newest stored snapshot and today's stored samples for an
// instant first frame.
func (s *Service) Seed(ctx context.Context) (provider.Snapshot, []provider.Sample, bool) {
	if s.store == nil {
		return provider.Snapshot{}, nil, false
	}
	snap, ok, err := s.store.LatestSnapshot(ctx)
	if err != nil || !ok {
		return provider.Snapshot{}, nil, false
	}
	samples, _ := s.localSamples(ctx, time.Now())
	return snap, samples, true
}

func (s *Service) localSamples(ctx context.Context, day time.Time) ([]provider.Sample, error) {
	if s.store == nil {
		return nil, nil
	}
	loc := day.Location()
	start := time.Date(day.Year(), day.Month(), day.Day(), 0, 0, 0, 0, loc)
	return s.store.Samples(ctx, start, start.AddDate(0, 0, 1))
}

// mergeSamples merges by timestamp. Local samples win when both sides have a
// value at the same instant, because they are recorded from the exact device
// reading rather than the cloud's chart endpoint.
func mergeSamples(remote, local []provider.Sample) []provider.Sample {
	byTS := make(map[int64]provider.Sample, len(remote)+len(local))
	for _, s := range remote {
		byTS[s.TS.UnixMilli()] = s
	}
	for _, s := range local {
		byTS[s.TS.UnixMilli()] = s
	}
	out := make([]provider.Sample, 0, len(byTS))
	for _, s := range byTS {
		out = append(out, s)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].TS.Before(out[j].TS) })
	return out
}

func mergeAlarms(remote, local []provider.Alarm) []provider.Alarm {
	byID := make(map[string]provider.Alarm, len(remote)+len(local))
	for _, a := range remote {
		byID[a.ID] = a
	}
	for _, a := range local {
		if a.ID == "" {
			a.ID = fmt.Sprintf("%s-%d", a.Code, a.Begin.Unix())
		}
		if _, exists := byID[a.ID]; !exists {
			byID[a.ID] = a
		}
	}
	out := make([]provider.Alarm, 0, len(byID))
	for _, a := range byID {
		out = append(out, a)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Begin.After(out[j].Begin) })
	return out
}
