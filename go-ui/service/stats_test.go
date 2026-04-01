package service

import (
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
