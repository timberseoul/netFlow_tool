package service

import (
	"log"
	"netFlow_tool-ui/ipc"
	"netFlow_tool-ui/types"
	"sync"
	"time"
)

const throughputRetention = 30 * time.Minute

type throughputSample struct {
	timestamp     time.Time
	uploadSpeed   float64
	downloadSpeed float64
}

type flowSnapshotSample struct {
	timestamp time.Time
	stats     []types.ProcessFlow
}

// StatsService runs an independent goroutine that polls the Rust core
// at a fixed interval using time.Ticker (true wall-clock cadence, no drift).
// The UI reads cached results via Snapshot() — zero coupling between
// IPC latency and UI refresh rate.
type StatsService struct {
	client     *ipc.Client
	mu         sync.RWMutex
	stats      []types.ProcessFlow
	history    []types.DailyUsage
	throughput []throughputSample
	flowCache  []flowSnapshotSample
	lastErr    error
	historyErr error
	interval   time.Duration
	stopCh     chan struct{}
}

// NewStatsService creates a new stats polling service.
func NewStatsService(client *ipc.Client, interval time.Duration) *StatsService {
	return &StatsService{
		client:   client,
		interval: interval,
		stopCh:   make(chan struct{}),
	}
}

// Start begins the background polling goroutine.
// The goroutine fetches stats immediately, then every `interval`.
func (s *StatsService) Start() {
	// Do an initial fetch so the UI has data right away.
	s.poll()
	s.pollHistory()

	go func() {
		ticker := time.NewTicker(s.interval)
		defer ticker.Stop()
		historyTicker := time.NewTicker(time.Minute)
		defer historyTicker.Stop()

		for {
			select {
			case <-s.stopCh:
				return
			case <-ticker.C:
				s.poll()
			case <-historyTicker.C:
				s.pollHistory()
			}
		}
	}()
}

// poll fetches stats from the Rust core and stores the result.
func (s *StatsService) poll() {
	stats, err := s.client.GetStats()
	s.mu.Lock()
	if err != nil {
		log.Printf("StatsService: poll error: %v", err)
		s.lastErr = err
		// Keep stale stats so the UI still shows the last good data.
	} else {
		now := time.Now()
		s.stats = stats
		s.recordThroughputSampleLocked(now, stats)
		s.recordFlowSnapshotLocked(now, stats)
		s.lastErr = nil
	}
	s.mu.Unlock()
}

// pollHistory fetches persisted daily usage history from the Rust core.
func (s *StatsService) pollHistory() {
	history, err := s.client.GetHistory()
	s.mu.Lock()
	if err != nil {
		log.Printf("StatsService: history poll error: %v", err)
		s.historyErr = err
	} else {
		s.history = history
		s.historyErr = nil
	}
	s.mu.Unlock()
}

// Snapshot returns a copy of the latest stats and the last error (if any).
// This is called by the UI on every tick — it is non-blocking and O(n).
func (s *StatsService) Snapshot() ([]types.ProcessFlow, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	result := make([]types.ProcessFlow, len(s.stats))
	copy(result, s.stats)
	return result, s.lastErr
}

// SnapshotHistory returns a copy of the latest persisted history.
func (s *StatsService) SnapshotHistory() ([]types.DailyUsage, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	result := make([]types.DailyUsage, len(s.history))
	copy(result, s.history)
	return result, s.historyErr
}

// SnapshotThroughput returns averaged throughput points derived from cached samples.
func (s *StatsService) SnapshotThroughput(window time.Duration, maxPoints int) []types.ThroughputPoint {
	s.mu.RLock()
	samples := make([]throughputSample, len(s.throughput))
	copy(samples, s.throughput)
	s.mu.RUnlock()

	return aggregateThroughputSamples(samples, window, maxPoints)
}

// SnapshotAveragedFlows returns averaged process speeds from the newest cached window.
func (s *StatsService) SnapshotAveragedFlows(window time.Duration) []types.ProcessFlow {
	s.mu.RLock()
	samples := make([]flowSnapshotSample, len(s.flowCache))
	copy(samples, s.flowCache)
	latest := make([]types.ProcessFlow, len(s.stats))
	copy(latest, s.stats)
	s.mu.RUnlock()

	averaged := aggregateFlowSnapshots(samples, window)
	if len(averaged) > 0 {
		return averaged
	}
	return latest
}

// Stop stops the polling goroutine.
func (s *StatsService) Stop() {
	close(s.stopCh)
}

func (s *StatsService) recordThroughputSampleLocked(now time.Time, stats []types.ProcessFlow) {
	uploadSpeed, downloadSpeed := sumThroughput(stats)
	s.throughput = append(s.throughput, throughputSample{
		timestamp:     now,
		uploadSpeed:   uploadSpeed,
		downloadSpeed: downloadSpeed,
	})

	cutoff := now.Add(-throughputRetention)
	trimIndex := 0
	for trimIndex < len(s.throughput) && s.throughput[trimIndex].timestamp.Before(cutoff) {
		trimIndex++
	}
	if trimIndex > 0 {
		s.throughput = append([]throughputSample(nil), s.throughput[trimIndex:]...)
	}
}

func (s *StatsService) recordFlowSnapshotLocked(now time.Time, stats []types.ProcessFlow) {
	snapshot := make([]types.ProcessFlow, len(stats))
	copy(snapshot, stats)
	s.flowCache = append(s.flowCache, flowSnapshotSample{
		timestamp: now,
		stats:     snapshot,
	})

	cutoff := now.Add(-throughputRetention)
	trimIndex := 0
	for trimIndex < len(s.flowCache) && s.flowCache[trimIndex].timestamp.Before(cutoff) {
		trimIndex++
	}
	if trimIndex > 0 {
		s.flowCache = append([]flowSnapshotSample(nil), s.flowCache[trimIndex:]...)
	}
}

func sumThroughput(stats []types.ProcessFlow) (float64, float64) {
	var uploadSpeed float64
	var downloadSpeed float64
	for _, flow := range stats {
		uploadSpeed += flow.UploadSpeed
		downloadSpeed += flow.DownloadSpeed
	}
	return uploadSpeed, downloadSpeed
}

func aggregateThroughputSamples(samples []throughputSample, window time.Duration, maxPoints int) []types.ThroughputPoint {
	if len(samples) == 0 || window <= 0 || maxPoints <= 0 {
		return nil
	}

	type bucket struct {
		start       time.Time
		uploadSum   float64
		downloadSum float64
		sampleCount int
	}

	buckets := make([]bucket, 0, len(samples))
	for _, sample := range samples {
		start := sample.timestamp.Truncate(window)
		if len(buckets) == 0 || !buckets[len(buckets)-1].start.Equal(start) {
			buckets = append(buckets, bucket{start: start})
		}

		current := &buckets[len(buckets)-1]
		current.uploadSum += sample.uploadSpeed
		current.downloadSum += sample.downloadSpeed
		current.sampleCount++
	}

	if len(buckets) > maxPoints {
		buckets = buckets[len(buckets)-maxPoints:]
	}

	points := make([]types.ThroughputPoint, 0, len(buckets))
	for _, bucket := range buckets {
		if bucket.sampleCount == 0 {
			continue
		}

		points = append(points, types.ThroughputPoint{
			Label:         bucket.start.Format("15:04"),
			Timestamp:     bucket.start.Format(time.RFC3339),
			UploadSpeed:   bucket.uploadSum / float64(bucket.sampleCount),
			DownloadSpeed: bucket.downloadSum / float64(bucket.sampleCount),
			SampleCount:   bucket.sampleCount,
		})
	}

	return points
}

func aggregateFlowSnapshots(samples []flowSnapshotSample, window time.Duration) []types.ProcessFlow {
	if len(samples) == 0 || window <= 0 {
		return nil
	}

	latestBucketStart := samples[len(samples)-1].timestamp.Truncate(window)
	bucketSamples := make([]flowSnapshotSample, 0, len(samples))
	for _, sample := range samples {
		if sample.timestamp.Truncate(window).Equal(latestBucketStart) {
			bucketSamples = append(bucketSamples, sample)
		}
	}
	if len(bucketSamples) == 0 {
		return nil
	}

	type aggregate struct {
		flow          types.ProcessFlow
		uploadSum     float64
		downloadSum   float64
		latestSeen    time.Time
		latestPresent bool
	}

	bucketSampleCount := float64(len(bucketSamples))
	aggregates := make(map[uint32]*aggregate)
	order := make([]uint32, 0)
	for _, sample := range bucketSamples {
		for _, flow := range sample.stats {
			item, exists := aggregates[flow.PID]
			if !exists {
				flowCopy := flow
				item = &aggregate{flow: flowCopy}
				aggregates[flow.PID] = item
				order = append(order, flow.PID)
			}

			item.uploadSum += flow.UploadSpeed
			item.downloadSum += flow.DownloadSpeed
			if !item.latestPresent || sample.timestamp.After(item.latestSeen) {
				item.flow = flow
				item.latestSeen = sample.timestamp
				item.latestPresent = true
			}
			if flow.TotalUpload > item.flow.TotalUpload {
				item.flow.TotalUpload = flow.TotalUpload
			}
			if flow.TotalDownload > item.flow.TotalDownload {
				item.flow.TotalDownload = flow.TotalDownload
			}
		}
	}

	result := make([]types.ProcessFlow, 0, len(order))
	for _, pid := range order {
		item := aggregates[pid]
		flow := item.flow
		flow.UploadSpeed = item.uploadSum / bucketSampleCount
		flow.DownloadSpeed = item.downloadSum / bucketSampleCount
		result = append(result, flow)
	}

	return result
}
