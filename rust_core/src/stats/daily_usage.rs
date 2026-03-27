use std::fs::{self, File};
use std::io::{self, BufReader, BufWriter, Write};
use std::path::{Path, PathBuf};
use std::time::Instant;

use chrono::{Local, NaiveDate};
use log::warn;
use serde::{Deserialize, Serialize};

const MAX_DAYS: usize = 90;
const FLUSH_INTERVAL_SECS: u64 = 60;

#[derive(Debug, Clone, Serialize, Deserialize)]
pub struct DailyUsageRecord {
    pub date: String,
    pub upload: u64,
    pub download: u64,
}

#[derive(Debug, Serialize, Deserialize)]
struct PersistedDailyUsage {
    current_day: String,
    current_upload: u64,
    current_download: u64,
    history: Vec<DailyUsageRecord>,
}

pub struct DailyUsageStore {
    path: PathBuf,
    current_day: NaiveDate,
    current_upload: u64,
    current_download: u64,
    history: Vec<DailyUsageRecord>,
    dirty: bool,
    last_flush: Instant,
}

impl DailyUsageStore {
    pub fn load() -> Self {
        let path = default_storage_path();
        let today = today_local();

        let mut store = Self {
            path,
            current_day: today,
            current_upload: 0,
            current_download: 0,
            history: Vec::new(),
            dirty: false,
            last_flush: Instant::now(),
        };

        if let Err(err) = store.load_from_disk() {
            warn!("Failed to load daily usage file: {}", err);
        }

        if store.current_day < today {
            // 兼容旧数据格式：若文件中的 current_* 还未实时同步进 history，先补一次。
            store.archive_current_day();
            store.current_day = today;
            store.current_upload = 0;
            store.current_download = 0;
            store.dirty = true;
        } else if store.current_day > today {
            warn!(
                "Daily usage file has future date {}; resetting to today {}",
                store.current_day, today
            );
            store.current_day = today;
            store.current_upload = 0;
            store.current_download = 0;
            store.dirty = true;
        }

        store.sync_current_day_into_history();

        if store.dirty {
            let _ = store.flush_now();
        }

        store
    }

    pub fn record(&mut self, upload: u64, download: u64) {
        if upload == 0 && download == 0 {
            return;
        }

        self.current_upload += upload;
        self.current_download += download;
        self.sync_current_day_into_history();
        self.dirty = true;
    }

    pub fn maybe_rollover(&mut self) -> bool {
        let today = today_local();
        if today <= self.current_day {
            return false;
        }

        self.current_day = today;
        self.current_upload = 0;
        self.current_download = 0;
        self.sync_current_day_into_history();
        self.dirty = true;

        if let Err(err) = self.flush_now() {
            warn!("Failed to flush daily usage after rollover: {}", err);
        }

        true
    }

    pub fn maybe_flush(&mut self) {
        if !self.dirty || self.last_flush.elapsed().as_secs() < FLUSH_INTERVAL_SECS {
            return;
        }

        if let Err(err) = self.flush_now() {
            warn!("Failed to flush daily usage file: {}", err);
        }
    }

    pub fn flush_now(&mut self) -> io::Result<()> {
        if let Some(parent) = self.path.parent() {
            fs::create_dir_all(parent)?;
        }

        self.sync_current_day_into_history();

        let state = PersistedDailyUsage {
            current_day: self.current_day.format("%Y-%m-%d").to_string(),
            current_upload: self.current_upload,
            current_download: self.current_download,
            history: self.history.clone(),
        };

        let tmp_path = self.path.with_extension("json.tmp");
        {
            let file = File::create(&tmp_path)?;
            let mut writer = BufWriter::new(file);
            serde_json::to_writer_pretty(&mut writer, &state)?;
            writer.write_all(b"\n")?;
            writer.flush()?;
            writer.get_ref().sync_all()?;
        }

        atomic_replace(&tmp_path, &self.path)?;
        self.dirty = false;
        self.last_flush = Instant::now();
        Ok(())
    }

    pub fn snapshot(&self) -> Vec<DailyUsageRecord> {
        let mut rows = self.history.clone();
        rows.sort_by(|a, b| b.date.cmp(&a.date));
        rows.truncate(MAX_DAYS);
        rows
    }

    fn load_from_disk(&mut self) -> io::Result<()> {
        if !self.path.exists() {
            return Ok(());
        }

        let file = File::open(&self.path)?;
        let reader = BufReader::new(file);
        let state = match serde_json::from_reader::<_, PersistedDailyUsage>(reader) {
            Ok(state) => state,
            Err(err) => {
                self.backup_corrupted_file();
                return Err(io::Error::new(io::ErrorKind::InvalidData, err));
            }
        };

        self.current_day = NaiveDate::parse_from_str(&state.current_day, "%Y-%m-%d")
            .unwrap_or_else(|_| today_local());
        self.current_upload = state.current_upload;
        self.current_download = state.current_download;
        self.history = normalize_history(state.history);
        self.sync_current_day_into_history();
        Ok(())
    }

    fn archive_current_day(&mut self) {
        if self.current_upload == 0 && self.current_download == 0 {
            return;
        }

        self.sync_current_day_into_history();
    }

    fn sync_current_day_into_history(&mut self) {
        let row = DailyUsageRecord {
            date: self.current_day.format("%Y-%m-%d").to_string(),
            upload: self.current_upload,
            download: self.current_download,
        };
        upsert_history(&mut self.history, row);
        trim_history(&mut self.history);
    }

    fn backup_corrupted_file(&self) {
        let backup = self.path.with_extension(format!(
            "corrupt-{}.json",
            Local::now().format("%Y%m%d%H%M%S")
        ));
        if let Err(err) = fs::rename(&self.path, &backup) {
            warn!("Failed to backup corrupted daily usage file: {}", err);
        }
    }
}

fn default_storage_path() -> PathBuf {
    let exe_dir = std::env::current_exe()
        .ok()
        .and_then(|p| p.parent().map(|dir| dir.to_path_buf()))
        .unwrap_or_else(|| PathBuf::from("."));
    exe_dir.join("data").join("daily_totals.json")
}

fn normalize_history(history: Vec<DailyUsageRecord>) -> Vec<DailyUsageRecord> {
    let mut merged: Vec<DailyUsageRecord> = Vec::new();
    for row in history {
        upsert_history(&mut merged, row);
    }
    trim_history(&mut merged);
    merged
}

fn upsert_history(history: &mut Vec<DailyUsageRecord>, row: DailyUsageRecord) {
    if let Some(existing) = history.iter_mut().find(|item| item.date == row.date) {
        existing.upload = row.upload;
        existing.download = row.download;
    } else {
        history.push(row);
    }
    history.sort_by(|a, b| b.date.cmp(&a.date));
}

fn trim_history(history: &mut Vec<DailyUsageRecord>) {
    history.sort_by(|a, b| b.date.cmp(&a.date));
    history.truncate(MAX_DAYS);
}

fn today_local() -> NaiveDate {
    Local::now().date_naive()
}

#[cfg(windows)]
fn atomic_replace(src: &Path, dst: &Path) -> io::Result<()> {
    use std::os::windows::ffi::OsStrExt;

    use windows::core::PCWSTR;
    use windows::Win32::Storage::FileSystem::{
        MoveFileExW, MOVEFILE_REPLACE_EXISTING, MOVEFILE_WRITE_THROUGH,
    };

    let src_w: Vec<u16> = src.as_os_str().encode_wide().chain(Some(0)).collect();
    let dst_w: Vec<u16> = dst.as_os_str().encode_wide().chain(Some(0)).collect();

    unsafe {
        MoveFileExW(
            PCWSTR(src_w.as_ptr()),
            PCWSTR(dst_w.as_ptr()),
            MOVEFILE_REPLACE_EXISTING | MOVEFILE_WRITE_THROUGH,
        )
        .map_err(|_| io::Error::last_os_error())
    }
}
