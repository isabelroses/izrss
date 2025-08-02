use anyhow::Result;
use std::path::PathBuf;

fn get_state_dir() -> Result<PathBuf> {
    let state_dir = user_dirs::data_dir()?.join("izrss");

    Ok(state_dir)
}

pub fn read_state() -> Result<crate::feeds::Feeds> {
    let state_file = get_state_dir()?.join("state.json");

    let state_data = std::fs::read_to_string(state_file)?;
    let items: Vec<crate::feeds::Feed> = serde_json::from_str(&state_data)?;

    Ok(items)
}

impl crate::App<'_> {
    pub fn write_state(&self) -> Result<()> {
        let state_dir = get_state_dir()?;
        std::fs::create_dir_all(&state_dir)?;

        let state_file = state_dir.join("state.json");
        let state_data = serde_json::to_string(&self.feeds.items)?;
        std::fs::write(state_file, state_data)?;

        Ok(())
    }
}
