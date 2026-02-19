package actions

import (
	"strconv"
	"sync"
	"time"

	"github.com/ava-labs/avalanchego/ids"
)

const defaultZKMetricsMaxWindows = 200_000

type ZKWindowMetrics struct {
	MarketID string `json:"market_id"`
	WindowID uint64 `json:"window_id"`

	BatchSizeHint uint32 `json:"batch_size_hint"`

	CommitCount uint32 `json:"commit_count"`
	RevealCount uint32 `json:"reveal_count"`

	FirstCommitAtMs int64 `json:"first_commit_at_ms"`
	LastCommitAtMs  int64 `json:"last_commit_at_ms"`
	WindowCloseAtMs int64 `json:"window_close_at_ms"`

	ProofSubmittedAtMs int64 `json:"proof_submitted_at_ms"`
	ClearAcceptedAtMs  int64 `json:"clear_accepted_at_ms"`

	WitnessBuildMs      int64 `json:"witness_build_ms"`
	ProofGenerationMs   int64 `json:"proof_generation_ms"`
	ProofVerificationMs int64 `json:"proof_verification_ms"`

	BatchFreezeMs        int64 `json:"batch_freeze_ms"`
	ProofSubmitLatencyMs int64 `json:"proof_submit_latency_ms"`
	BlockAcceptLatencyMs int64 `json:"block_accept_latency_ms"`

	CommitExecUs      uint64 `json:"commit_exec_us"`
	RevealExecUs      uint64 `json:"reveal_exec_us"`
	ProofSubmitExecUs uint64 `json:"proof_submit_exec_us"`
	ClearExecUs       uint64 `json:"clear_exec_us"`

	MissedDeadline bool   `json:"missed_deadline"`
	Rejected       bool   `json:"rejected"`
	LastError      string `json:"last_error,omitempty"`
}

type ZKMetricsSummary struct {
	TotalWindowsObserved       uint64 `json:"total_windows_observed"`
	TotalCommits               uint64 `json:"total_commits"`
	TotalReveals               uint64 `json:"total_reveals"`
	TotalProofSubmissions      uint64 `json:"total_proof_submissions"`
	TotalProofSubmissionErrors uint64 `json:"total_proof_submission_errors"`
	TotalClears                uint64 `json:"total_clears"`
	TotalClearErrors           uint64 `json:"total_clear_errors"`
	TotalAcceptedBatches       uint64 `json:"total_accepted_batches"`
	TotalRejectedBatches       uint64 `json:"total_rejected_batches"`
	TotalMissedProofDeadlines  uint64 `json:"total_missed_proof_deadlines"`
}

type ZKMetricsSnapshot struct {
	GeneratedAtMs int64             `json:"generated_at_ms"`
	Summary       ZKMetricsSummary  `json:"summary"`
	Windows       []ZKWindowMetrics `json:"windows,omitempty"`
}

type zkMetricsCollector struct {
	mu         sync.Mutex
	maxWindows int
	windows    map[string]*ZKWindowMetrics
	order      []string

	summary ZKMetricsSummary
}

func newZKMetricsCollector(maxWindows int) *zkMetricsCollector {
	return &zkMetricsCollector{
		maxWindows: maxWindows,
		windows:    make(map[string]*ZKWindowMetrics, 1024),
		order:      make([]string, 0, 1024),
	}
}

func (c *zkMetricsCollector) reset() {
	c.mu.Lock()
	defer c.mu.Unlock()

	c.windows = make(map[string]*ZKWindowMetrics, 1024)
	c.order = c.order[:0]
	c.summary = ZKMetricsSummary{}
}

func zkWindowKey(marketID ids.ID, windowID uint64) string {
	return marketID.String() + ":" + strconv.FormatUint(windowID, 10)
}

func (c *zkMetricsCollector) getOrCreateLocked(marketID ids.ID, windowID uint64) *ZKWindowMetrics {
	key := zkWindowKey(marketID, windowID)
	if w, ok := c.windows[key]; ok {
		return w
	}
	if c.maxWindows > 0 && len(c.order) >= c.maxWindows {
		oldest := c.order[0]
		c.order = c.order[1:]
		delete(c.windows, oldest)
	}
	w := &ZKWindowMetrics{
		MarketID: marketID.String(),
		WindowID: windowID,
	}
	c.windows[key] = w
	c.order = append(c.order, key)
	c.summary.TotalWindowsObserved++
	return w
}

func (c *zkMetricsCollector) recordCommit(marketID ids.ID, windowID uint64, timestampMs int64, exec time.Duration, err error) {
	c.mu.Lock()
	defer c.mu.Unlock()

	w := c.getOrCreateLocked(marketID, windowID)
	if exec > 0 {
		w.CommitExecUs += uint64(exec.Microseconds())
	}
	if err != nil {
		w.Rejected = true
		w.LastError = err.Error()
		return
	}
	w.CommitCount++
	if w.FirstCommitAtMs == 0 || timestampMs < w.FirstCommitAtMs {
		w.FirstCommitAtMs = timestampMs
	}
	if timestampMs > w.LastCommitAtMs {
		w.LastCommitAtMs = timestampMs
	}
	c.summary.TotalCommits++
}

func (c *zkMetricsCollector) recordReveal(marketID ids.ID, windowID uint64, exec time.Duration, err error) {
	c.mu.Lock()
	defer c.mu.Unlock()

	w := c.getOrCreateLocked(marketID, windowID)
	if exec > 0 {
		w.RevealExecUs += uint64(exec.Microseconds())
	}
	if err != nil {
		w.Rejected = true
		w.LastError = err.Error()
		return
	}
	w.RevealCount++
	c.summary.TotalReveals++
}

func (c *zkMetricsCollector) recordProofSubmit(
	marketID ids.ID,
	windowID uint64,
	windowCloseAtMs int64,
	submittedAtMs int64,
	exec time.Duration,
	missedDeadline bool,
	err error,
) {
	c.mu.Lock()
	defer c.mu.Unlock()

	w := c.getOrCreateLocked(marketID, windowID)
	if exec > 0 {
		w.ProofSubmitExecUs += uint64(exec.Microseconds())
	}
	if windowCloseAtMs > 0 {
		w.WindowCloseAtMs = windowCloseAtMs
	}
	if submittedAtMs > 0 {
		w.ProofSubmittedAtMs = submittedAtMs
	}
	if missedDeadline {
		w.MissedDeadline = true
		c.summary.TotalMissedProofDeadlines++
	}
	if err != nil {
		w.Rejected = true
		w.LastError = err.Error()
		c.summary.TotalProofSubmissionErrors++
		return
	}
	c.summary.TotalProofSubmissions++
}

func (c *zkMetricsCollector) recordClear(
	marketID ids.ID,
	windowID uint64,
	acceptedAtMs int64,
	verifyDuration time.Duration,
	exec time.Duration,
	missedDeadline bool,
	err error,
) {
	c.mu.Lock()
	defer c.mu.Unlock()

	w := c.getOrCreateLocked(marketID, windowID)
	if exec > 0 {
		w.ClearExecUs += uint64(exec.Microseconds())
	}
	if verifyDuration > 0 {
		w.ProofVerificationMs = verifyDuration.Milliseconds()
	}
	if missedDeadline {
		w.MissedDeadline = true
		c.summary.TotalMissedProofDeadlines++
	}
	if err != nil {
		w.Rejected = true
		w.LastError = err.Error()
		c.summary.TotalClearErrors++
		c.summary.TotalRejectedBatches++
		return
	}

	w.ClearAcceptedAtMs = acceptedAtMs
	c.summary.TotalClears++
	c.summary.TotalAcceptedBatches++
}

func (c *zkMetricsCollector) recordProverStages(
	marketID ids.ID,
	windowID uint64,
	batchSizeHint uint32,
	witnessBuildMs int64,
	proofGenerationMs int64,
) {
	c.mu.Lock()
	defer c.mu.Unlock()

	w := c.getOrCreateLocked(marketID, windowID)
	if batchSizeHint > 0 {
		w.BatchSizeHint = batchSizeHint
	}
	if witnessBuildMs >= 0 {
		w.WitnessBuildMs = witnessBuildMs
	}
	if proofGenerationMs >= 0 {
		w.ProofGenerationMs = proofGenerationMs
	}
}

func (c *zkMetricsCollector) snapshot(limit int, includeWindows bool) ZKMetricsSnapshot {
	c.mu.Lock()
	defer c.mu.Unlock()

	snap := ZKMetricsSnapshot{
		GeneratedAtMs: time.Now().UnixMilli(),
		Summary:       c.summary,
	}
	if !includeWindows {
		return snap
	}

	keys := c.order
	if limit > 0 && len(keys) > limit {
		keys = keys[len(keys)-limit:]
	}
	snap.Windows = make([]ZKWindowMetrics, 0, len(keys))
	for _, key := range keys {
		w := c.windows[key]
		if w == nil {
			continue
		}
		cp := *w

		if cp.WindowCloseAtMs > 0 && cp.FirstCommitAtMs > 0 && cp.WindowCloseAtMs >= cp.FirstCommitAtMs {
			cp.BatchFreezeMs = cp.WindowCloseAtMs - cp.FirstCommitAtMs
		}
		if cp.WindowCloseAtMs > 0 && cp.ProofSubmittedAtMs >= cp.WindowCloseAtMs {
			cp.ProofSubmitLatencyMs = cp.ProofSubmittedAtMs - cp.WindowCloseAtMs
		}
		if cp.WindowCloseAtMs > 0 && cp.ClearAcceptedAtMs >= cp.WindowCloseAtMs {
			cp.BlockAcceptLatencyMs = cp.ClearAcceptedAtMs - cp.WindowCloseAtMs
		}

		snap.Windows = append(snap.Windows, cp)
	}
	return snap
}

var zkMetrics = newZKMetricsCollector(defaultZKMetricsMaxWindows)

func ResetZKMetrics() {
	zkMetrics.reset()
}

func RecordCommitMetric(marketID ids.ID, windowID uint64, timestampMs int64, exec time.Duration, err error) {
	zkMetrics.recordCommit(marketID, windowID, timestampMs, exec, err)
}

func RecordRevealMetric(marketID ids.ID, windowID uint64, exec time.Duration, err error) {
	zkMetrics.recordReveal(marketID, windowID, exec, err)
}

func RecordProofSubmitMetric(
	marketID ids.ID,
	windowID uint64,
	windowCloseAtMs int64,
	submittedAtMs int64,
	exec time.Duration,
	missedDeadline bool,
	err error,
) {
	zkMetrics.recordProofSubmit(
		marketID,
		windowID,
		windowCloseAtMs,
		submittedAtMs,
		exec,
		missedDeadline,
		err,
	)
}

func RecordClearMetric(
	marketID ids.ID,
	windowID uint64,
	acceptedAtMs int64,
	verifyDuration time.Duration,
	exec time.Duration,
	missedDeadline bool,
	err error,
) {
	zkMetrics.recordClear(
		marketID,
		windowID,
		acceptedAtMs,
		verifyDuration,
		exec,
		missedDeadline,
		err,
	)
}

func RecordProverStageMetrics(
	marketID ids.ID,
	windowID uint64,
	batchSizeHint uint32,
	witnessBuildMs int64,
	proofGenerationMs int64,
) {
	zkMetrics.recordProverStages(
		marketID,
		windowID,
		batchSizeHint,
		witnessBuildMs,
		proofGenerationMs,
	)
}

func GetZKMetricsSnapshot(limit int, includeWindows bool) ZKMetricsSnapshot {
	return zkMetrics.snapshot(limit, includeWindows)
}
