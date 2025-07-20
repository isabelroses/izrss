use anyhow::{Error, Result};
use std::{fs, path::PathBuf};

use serde::Deserialize;

fn get_config_file() -> Result<String> {
    let path = user_dirs::config_dir()?
        .join("izrss")
        .join("config.toml")
        .to_string_lossy()
        .to_string();

    Ok(path)
}

pub fn load_config(config_path: Option<&str>) -> Result<Config> {
    let path = match config_path {
        Some(p) => PathBuf::from(p),
        None => PathBuf::from(get_config_file()?),
    };

    let config_raw = fs::read_to_string(&path).unwrap_or_default();
    let config: Config = toml::from_str(&config_raw).map_err(Error::new)?;
    Ok(config)
}

#[derive(Deserialize, Debug, Clone)]
pub struct Config {
    #[serde(default = "default_home")]
    pub home: String,

    #[serde(default = "Colors::default")]
    pub colors: Colors,

    #[serde(default = "Reader::default")]
    pub reader: Reader,

    #[serde(default = "default_dateformat")]
    pub dateformat: String,

    // error if there are no URLs
    pub urls: Vec<String>,
}

fn default_home() -> String {
    "home".to_string()
}

fn default_dateformat() -> String {
    "02/01/2006".to_string()
}

impl Config {
    pub fn get_feed_urls(&self) -> Vec<String> {
        self.urls.clone()
    }

    pub fn get_text_color(&self) -> String {
        self.colors.text.clone()
    }

    pub fn get_invert_text_color(&self) -> String {
        self.colors.inverttext.clone()
    }

    pub fn get_subtext_color(&self) -> String {
        self.colors.subtext.clone()
    }

    pub fn get_accent_color(&self) -> String {
        self.colors.accent.clone()
    }

    pub fn get_borders_color(&self) -> String {
        self.colors.borders.clone()
    }
}

#[derive(Deserialize, Debug, Clone, Default)]
pub struct Colors {
    #[serde(default = "default_text_color")]
    pub text: String,

    #[serde(default = "default_invert_text_color")]
    pub inverttext: String,

    #[serde(default = "default_subtext_color")]
    pub subtext: String,

    #[serde(default = "default_accent_color")]
    pub accent: String,

    #[serde(default = "default_borders_color")]
    pub borders: String,
}

fn default_text_color() -> String {
    "#cdd6f4".to_string()
}

fn default_invert_text_color() -> String {
    "#1e1e2e".to_string()
}

fn default_subtext_color() -> String {
    "#a6adc8".to_string()
}

fn default_accent_color() -> String {
    "#74c7ec".to_string()
}

fn default_borders_color() -> String {
    "#313244".to_string()
}

impl Colors {
    pub fn default() -> Self {
        Self {
            text: default_text_color(),
            inverttext: default_invert_text_color(),
            subtext: default_subtext_color(),
            accent: default_accent_color(),
            borders: default_borders_color(),
        }
    }
}

// Configuration for the reader
#[derive(Deserialize, Debug, Clone)]
pub struct Reader {
    #[serde(default = "default_reader_size")]
    pub size: Size,

    #[serde(default = "default_theme")]
    pub theme: Option<String>,

    #[serde(default = "default_read_threshold")]
    pub read_threshold: f64,
}

fn default_reader_size() -> Size {
    Size::Recommended
}

fn default_theme() -> Option<String> {
    None
}

fn default_read_threshold() -> f64 {
    0.8
}

impl Default for Reader {
    fn default() -> Self {
        Self {
            size: default_reader_size(),
            theme: default_theme(),
            read_threshold: default_read_threshold(),
        }
    }
}

#[derive(Deserialize, Debug, Clone)]
#[serde(untagged)]
pub enum Size {
    String(String),
    Recommended,
}
