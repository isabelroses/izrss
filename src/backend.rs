use anyhow::Result;
use hex::encode;
use sha2::{Digest, Sha256};
use std::{
    path::PathBuf,
    time::{Duration, SystemTime},
};
use tokio::fs as tokio_fs;

const CACHE_TTL: Duration = Duration::from_secs(60 * 60); // 1 hour

fn cache_dir() -> Result<PathBuf> {
    let cache_dir = user_dirs::cache_dir()?.join("izrss");
    Ok(cache_dir)
}

fn url_to_cache_path(url: &str) -> Result<PathBuf> {
    let mut hasher = Sha256::new();
    hasher.update(url.as_bytes());
    let hash = encode(hasher.finalize());
    Ok(cache_dir()?.join(hash))
}

pub async fn fetch_with_cache(url: &str) -> Result<String> {
    let path = url_to_cache_path(url)?;

    if let Ok(metadata) = tokio_fs::metadata(&path).await {
        let modified = metadata.modified()?;
        let age = SystemTime::now().duration_since(modified)?;

        if age < CACHE_TTL {
            if let Ok(cached) = tokio_fs::read_to_string(&path).await {
                return Ok(cached);
            }
        }
    }

    let cache_dir = cache_dir()?;

    let body = reqwest::get(url).await?.text().await?;
    tokio_fs::create_dir_all(cache_dir).await.ok();
    tokio_fs::write(&path, &body).await?;
    Ok(body)
}
