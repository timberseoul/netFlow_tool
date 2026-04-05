package service

import (
	"netFlow_tool-ui/types"
	"testing"
	"time"
)

func TestAggregateThroughputSamplesAveragesPerBucket(t *testing.T) {
	base := time.Date(2026, 4, 1, 12, 0, 0, 0, time.UTC)
	samples := []throughputSample{
		{timestamp: base.Add(10 * time.Second), uploadSpeed: 10, downloadSpeed: 30},
		{timestamp: base.Add(70 * time.Second), uploadSpeed: 20, downloadSpeed: 50},
		{timestamp: base.Add(130 * time.Second), uploadSpeed: 40, downloadSpeed: 80},
		{timestamp: base.Add(170 * time.Second), uploadSpeed: 60, downloadSpeed: 100},
	}

	points := aggregateThroughputSamples(samples, 2*time.Minute, 10)
	if len(points) != 2 {
		t.Fatalf("expected 2 averaged points, got %d", len(points))
	}

	if points[0].UploadSpeed != 15 || points[0].DownloadSpeed != 40 || points[0].SampleCount != 2 {
		t.Fatalf("unexpected first bucket: %+v", points[0])
	}

	if points[1].UploadSpeed != 50 || points[1].DownloadSpeed != 90 || points[1].SampleCount != 2 {
		t.Fatalf("unexpected second bucket: %+v", points[1])
	}
}

func TestAggregateThroughputSamplesKeepsNewestPoints(t *testing.T) {
	base := time.Date(2026, 4, 1, 12, 0, 0, 0, time.UTC)
	samples := []throughputSample{
		{timestamp: base.Add(0), uploadSpeed: 10, downloadSpeed: 10},
		{timestamp: base.Add(2 * time.Minute), uploadSpeed: 20, downloadSpeed: 20},
		{timestamp: base.Add(4 * time.Minute), uploadSpeed: 30, downloadSpeed: 30},
	}

	points := aggregateThroughputSamples(samples, 2*time.Minute, 2)
	if len(points) != 2 {
		t.Fatalf("expected 2 newest points, got %d", len(points))
	}

	if points[0].UploadSpeed != 20 || points[1].UploadSpeed != 30 {
		t.Fatalf("unexpected retained points: %+v", points)
	}
}

func TestAggregateFlowSnapshotsUsesNewestBucketAverage(t *testing.T) {
	base := time.Date(2026, 4, 1, 12, 0, 0, 0, time.UTC)
	parent := uint32(100)
	samples := []flowSnapshotSample{
		{
			timestamp: base.Add(10 * time.Second),
			stats: []types.ProcessFlow{
				{PID: 1, Name: "alpha", UploadSpeed: 10, DownloadSpeed: 30, TotalUpload: 100, TotalDownload: 300},
			},
		},
		{
			timestamp: base.Add(70 * time.Second),
			stats: []types.ProcessFlow{
				{PID: 1, Name: "alpha", UploadSpeed: 20, DownloadSpeed: 50, TotalUpload: 120, TotalDownload: 340},
				{PID: 2, ParentPID: &parent, Name: "beta", UploadSpeed: 40, DownloadSpeed: 60, TotalUpload: 90, TotalDownload: 150},
			},
		},
		{
			timestamp: base.Add(130 * time.Second),
			stats: []types.ProcessFlow{
				{PID: 1, Name: "alpha", UploadSpeed: 30, DownloadSpeed: 90, TotalUpload: 150, TotalDownload: 420},
				{PID: 2, ParentPID: &parent, Name: "beta", UploadSpeed: 20, DownloadSpeed: 40, TotalUpload: 110, TotalDownload: 190},
			},
		},
		{
			timestamp: base.Add(170 * time.Second),
			stats: []types.ProcessFlow{
				{PID: 1, Name: "alpha", UploadSpeed: 50, DownloadSpeed: 110, TotalUpload: 180, TotalDownload: 460},
			},
		},
	}

	flows := aggregateFlowSnapshots(samples, 2*time.Minute)
	if len(flows) != 2 {
		t.Fatalf("expected 2 flows from newest bucket, got %d", len(flows))
	}

	if flows[0].PID != 1 || flows[0].UploadSpeed != 40 || flows[0].DownloadSpeed != 100 {
		t.Fatalf("unexpected averaged parent flow: %+v", flows[0])
	}

	if flows[1].PID != 2 || flows[1].UploadSpeed != 10 || flows[1].DownloadSpeed != 20 {
		t.Fatalf("unexpected averaged child flow: %+v", flows[1])
	}

	if flows[1].ParentPID == nil || *flows[1].ParentPID != parent {
		t.Fatalf("expected parent pid to be preserved, got %+v", flows[1])
	}
}
