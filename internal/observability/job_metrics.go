package observability

import (
	"sync/atomic"
	"time"
)

type JobMetrics struct {
	claimed      atomic.Uint64
	done         atomic.Uint64
	failed       atomic.Uint64
	retried      atomic.Uint64
	deadLettered atomic.Uint64

	// duration stats (nanoseconds)
	durationCount atomic.Uint64
	durationTotal atomic.Int64
	durationMax   atomic.Int64
}

func NewJobMetrics() *JobMetrics {
	m := &JobMetrics{}
	m.durationMax.Store(0)
	return m
}

func (m *JobMetrics) IncClaimed() {
	m.claimed.Add(1)
}
func (m *JobMetrics) IncDone() {
	m.done.Add(1)
}
func (m *JobMetrics) IncFailed() {
	m.failed.Add(1)
}

func (m *JobMetrics) IncRetried() {
	m.retried.Add(1)
}

func (m *JobMetrics) IncDeadLettered() {
	m.deadLettered.Add(1)
}

func (m *JobMetrics) ObserveDuration(d time.Duration) {
	ns := d.Nanoseconds()
	m.durationCount.Add(1)
	m.durationTotal.Add(ns)

	// max update

	for {
		curr := m.durationMax.Load()

		if ns <= curr {
			return
		}

		if m.durationMax.CompareAndSwap(curr, ns) {
			return
		}
	}
}

type JobMetricsSnapShot struct {
	Claimed         uint64
	Done            uint64
	Failed          uint64
	Retried         uint64
	DeadLettered    uint64
	DurationCount   uint64
	AverageDuration time.Duration
	MaxDuration     time.Duration
}

func (m *JobMetrics) Snapshot() JobMetricsSnapShot {
	count := m.durationCount.Load()
	total := m.durationTotal.Load()
	max := m.durationMax.Load()

	var avg time.Duration

	if count > 0 {
		avg = time.Duration(total / int64(count))
	}

	return JobMetricsSnapShot{
		Claimed:         m.claimed.Load(),
		Done:            m.done.Load(),
		Failed:          m.failed.Load(),
		Retried:         m.retried.Load(),
		DeadLettered:    m.deadLettered.Load(),
		DurationCount:   count,
		AverageDuration: avg,
		MaxDuration:     time.Duration(max),
	}

}
