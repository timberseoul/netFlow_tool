use std::sync::Arc;
use std::time::{Duration, Instant};

use rustc_hash::FxHashMap;
use serde::Serialize;

/// Process category for grouping in the UI.
#[derive(Debug, Clone, Copy, Serialize, PartialEq, Eq, Hash)]
pub enum ProcessCategory {
    #[serde(rename = "user")]
    User,
    #[serde(rename = "system")]
    System,
    #[serde(rename = "service")]
    Service,
    #[serde(rename = "unknown")]
    Unknown,
}

/// Process activity status.
#[derive(Debug, Clone, Copy, Serialize, PartialEq, Eq)]
pub enum ProcessStatus {
    #[serde(rename = "active")]
    Active,
    #[serde(rename = "inactive")]
    Inactive,
}

/// Per-process state cache entry.
/// This is the global state for one PID — never deleted, only updated.
#[derive(Debug, Clone)]
struct ProcessEntry {
    pid: u32,
    parent_pid: Option<u32>,
    name: Arc<str>,
    category: ProcessCategory,
    /// Bytes accumulated since last speed reset (used to compute speed)
    upload_delta: u64,
    download_delta: u64,
    /// Lifetime cumulative bytes
    total_upload: u64,
    total_download: u64,
    /// When this process last had any traffic
    last_seen: Instant,
}

/// Snapshot sent to UI via IPC — serialised to JSON.
#[derive(Debug, Clone, Serialize)]
pub struct ProcessStats {
    pub pid: u32,
    #[serde(skip_serializing_if = "Option::is_none")]
    pub parent_pid: Option<u32>,
    pub name: String,
    pub category: ProcessCategory,
    pub status: ProcessStatus,
    pub upload_speed: f64,
    pub download_speed: f64,
    pub total_upload: u64,
    pub total_download: u64,
}

/// The global state cache:  packet → update cache → UI reads cache.
pub struct FlowAggregator {
    entries: FxHashMap<u32, ProcessEntry>,
    last_reset: Instant,
}

impl FlowAggregator {
    const INACTIVE_THRESHOLD: Duration = Duration::from_secs(30);
    const EXITED_RETENTION: Duration = Duration::from_secs(90);
    const ABSOLUTE_RETENTION: Duration = Duration::from_secs(10 * 60);

    pub fn new() -> Self {
        Self {
            entries: FxHashMap::default(),
            last_reset: Instant::now(),
        }
    }

    // ──────────────────────────────────────────
    //  Called on every captured packet
    // ──────────────────────────────────────────

    /// Accumulate bytes for a PID.  Never removes entries.
    /// `name` is `Arc<str>` — clone is O(1) ref-count bump.
    pub fn record(
        &mut self,
        pid: u32,
        parent_pid: Option<u32>,
        name: &Arc<str>,
        category: ProcessCategory,
        upload: u64,
        download: u64,
    ) {
        let now = Instant::now();
        let entry = self.entries.entry(pid).or_insert_with(|| ProcessEntry {
            pid,
            parent_pid,
            name: Arc::clone(name),
            category,
            upload_delta: 0,
            download_delta: 0,
            total_upload: 0,
            total_download: 0,
            last_seen: now,
        });

        // Keep name / category fresh (Arc pointer comparison first, cheap)
        if !Arc::ptr_eq(&entry.name, name) && *entry.name != **name {
            entry.name = Arc::clone(name);
        }
        entry.parent_pid = parent_pid;
        entry.category = category;

        // Accumulate delta (for speed calc) AND total (lifetime)
        entry.upload_delta += upload;
        entry.download_delta += download;
        entry.total_upload += upload;
        entry.total_download += download;

        // Mark active
        entry.last_seen = now;
    }

    // ──────────────────────────────────────────
    //  Called once per refresh cycle (e.g. 1 s)
    // ──────────────────────────────────────────

    /// Compute speeds from accumulated deltas, then reset deltas to 0.
    /// Returns a snapshot of ALL historically-seen processes.
    pub fn snapshot<F>(&mut self, mut is_process_alive: F) -> Vec<ProcessStats>
    where
        F: FnMut(u32) -> bool,
    {
        let now = Instant::now();
        let elapsed = now.duration_since(self.last_reset).as_secs_f64();
        let elapsed = if elapsed < 0.001 { 0.001 } else { elapsed };
        let mut stats = Vec::with_capacity(self.entries.len());
        self.entries.retain(|pid, entry| {
            let inactive_for = now.duration_since(entry.last_seen);
            let should_remove = inactive_for > Self::ABSOLUTE_RETENTION
                || (inactive_for > Self::EXITED_RETENTION && !is_process_alive(*pid));
            if should_remove {
                return false;
            }

            let status = if inactive_for > Self::INACTIVE_THRESHOLD {
                ProcessStatus::Inactive
            } else {
                ProcessStatus::Active
            };
            stats.push(ProcessStats {
                pid: entry.pid,
                parent_pid: entry.parent_pid,
                name: entry.name.to_string(),
                category: entry.category,
                status,
                upload_speed: entry.upload_delta as f64 / elapsed,
                download_speed: entry.download_delta as f64 / elapsed,
                total_upload: entry.total_upload,
                total_download: entry.total_download,
            });

            entry.upload_delta = 0;
            entry.download_delta = 0;
            true
        });

        self.last_reset = now;
        stats
    }
}

#[cfg(test)]
mod tests {
    use super::*;

    fn arc(value: &str) -> Arc<str> {
        Arc::from(value)
    }

    #[test]
    fn removes_exited_process_after_retention() {
        let mut aggregator = FlowAggregator::new();
        let name = arc("child.exe");
        aggregator.record(100, Some(50), &name, ProcessCategory::User, 128, 64);

        let entry = aggregator.entries.get_mut(&100).unwrap();
        entry.last_seen =
            Instant::now() - FlowAggregator::EXITED_RETENTION - Duration::from_secs(1);

        let stats = aggregator.snapshot(|_| false);
        assert!(stats.is_empty());
        assert!(!aggregator.entries.contains_key(&100));
    }

    #[test]
    fn keeps_alive_process_even_if_inactive_within_absolute_retention() {
        let mut aggregator = FlowAggregator::new();
        let name = arc("parent.exe");
        aggregator.record(42, None, &name, ProcessCategory::User, 256, 128);

        let entry = aggregator.entries.get_mut(&42).unwrap();
        entry.last_seen =
            Instant::now() - FlowAggregator::EXITED_RETENTION - Duration::from_secs(1);

        let stats = aggregator.snapshot(|pid| pid == 42);
        assert_eq!(stats.len(), 1);
        assert_eq!(stats[0].status, ProcessStatus::Inactive);
        assert_eq!(stats[0].parent_pid, None);
    }
}
