use anyhow::Result;
use ratatui::{
    buffer::Buffer,
    crossterm::event::{self, Event, KeyCode, KeyEvent, KeyEventKind},
    layout::Rect,
    style::{Color, Style},
    symbols::border,
    widgets::{
        Block, Borders, HighlightSpacing, List, ListItem, ListState, StatefulWidget, Widget,
    },
};
use std::str::FromStr;
use tokio::sync::mpsc::Receiver;

mod backend;
mod config;
mod feeds;
mod state;
mod tui;

#[tokio::main]
async fn main() -> Result<()> {
    let (tx, rx) = tokio::sync::mpsc::channel(16);

    let config = config::load_config(None)?;

    let config_clone = config.clone();
    tokio::spawn(async move {
        feeds::fetch_all_feeds(config_clone, tx).await.unwrap();
    });

    let mut terminal = tui::init()?;
    let app = App::new(rx, config);
    let app_result = app.run(&mut terminal);

    if let Err(err) = tui::restore() {
        eprintln!(
            "failed to restore terminal. Run `reset` or restart your terminal to recover: {err}",
        );
    }
    app_result
}

pub struct App {
    selected_feed: Option<feeds::Feed>,
    feeds: FeedsList,
    receiver: Receiver<feeds::Feed>,
    config: config::Config,
    exit: bool,
}

pub struct FeedsList {
    pub items: feeds::Feeds,
    pub state: ListState,
}

impl FeedsList {
    fn new(items: crate::feeds::Feeds) -> Self {
        Self {
            items,
            state: ListState::default(),
        }
    }
}

impl App {
    pub fn new(receiver: Receiver<feeds::Feed>, config: config::Config) -> Self {
        let feed_items = state::read_state().unwrap_or_default();

        Self {
            feeds: FeedsList::new(feed_items),
            selected_feed: None,
            config,
            receiver,
            exit: false,
        }
    }

    pub fn run(mut self, terminal: &mut tui::Tui) -> Result<()> {
        while !self.exit {
            terminal.draw(|frame| frame.render_widget(&mut self, frame.area()))?;
            self.handle_events()?
        }
        Ok(())
    }

    fn handle_events(&mut self) -> Result<()> {
        while let Ok(fetched) = self.receiver.try_recv() {
            if let Some(existing) = self.feeds.items.iter_mut().find(|f| f.url == fetched.url) {
                feeds::merge_fetched_feed(existing, fetched);
            } else {
                self.feeds.items.push(fetched);
            }

            let _ = self.write_state();
        }

        match event::read()? {
            // it's important to check that the event is a key press event as
            // crossterm also emits key release and repeat events on Windows.
            Event::Key(key_event) if key_event.kind == KeyEventKind::Press => {
                self.handle_key_event(key_event)
            }
            _ => Ok(()),
        }
    }

    fn handle_key_event(&mut self, key_event: KeyEvent) -> Result<()> {
        match key_event.code {
            KeyCode::Char('q') => self.exit()?,
            KeyCode::Char('j') | KeyCode::Down => self.select_next(),
            KeyCode::Char('k') | KeyCode::Up => self.select_previous(),
            KeyCode::Char('l') | KeyCode::Right => self.open_selected_feed(),
            _ => {}
        }
        Ok(())
    }

    fn select_next(&mut self) {
        self.feeds.state.select_next();
    }
    fn select_previous(&mut self) {
        self.feeds.state.select_previous();
    }
    fn open_selected_feed(&mut self) {
        self.selected_feed = {
            let id = self.feeds.state.selected();
            if let Some(id) = id {
                Some(self.feeds.items[id].clone())
            } else {
                None
            }
        };
    }

    fn exit(&mut self) -> Result<()> {
        self.write_state()?;
        self.exit = true;
        Ok(())
    }
}

impl Widget for &mut App {
    fn render(self, area: Rect, buf: &mut Buffer) {
        let block = Block::default()
            .borders(Borders::ALL)
            .border_set(border::ROUNDED);

        let items: Vec<ListItem> = self
            .feeds
            .items
            .iter()
            .map(|feed| {
                let title = format!("{}: {}", feed.title, feed.total_unread());
                ListItem::new(title)
            })
            .collect();

        let highlight_style = Style::new()
            .fg(Color::from_str(&self.config.get_invert_text_color()).unwrap())
            .bg(Color::from_str(&self.config.get_accent_color()).unwrap());

        let list = List::new(items)
            .block(block)
            .highlight_style(highlight_style)
            .highlight_spacing(HighlightSpacing::Always);

        StatefulWidget::render(list, area, buf, &mut self.feeds.state);
    }
}
