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
use std::time::Duration;
use tokio::sync::mpsc::Receiver;

mod backend;
mod config;
mod feeds;
mod reader;
mod state;
mod tui;

use self::reader::Reader;

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

pub struct App<'a> {
    view: View,
    feeds: FeedsList,
    selected_feed: PostsList,
    selected_post: Reader<'a>,
    selected_post_content: String,

    receiver: Receiver<feeds::Feed>,
    config: config::Config,
    exit: bool,
}

pub struct FeedsList {
    pub items: feeds::Feeds,
    pub state: ListState,
}

pub struct PostsList {
    pub items: Vec<feeds::Post>,
    pub state: ListState,
}

#[derive(Debug, Clone, Copy, PartialEq, Eq)]
pub enum View {
    Feeds,
    Posts,
    Post,
}

impl FeedsList {
    fn new(items: crate::feeds::Feeds) -> Self {
        Self {
            items,
            state: ListState::default().with_selected(Some(0)),
        }
    }
}

impl PostsList {
    fn new(items: Vec<feeds::Post>) -> Self {
        Self {
            items,
            state: ListState::default().with_selected(Some(0)),
        }
    }
}

impl<'a> App<'a> {
    pub fn new(receiver: Receiver<feeds::Feed>, config: config::Config) -> Self {
        let feed_items = state::read_state().unwrap_or_default();

        Self {
            view: View::Feeds,
            feeds: FeedsList::new(feed_items),
            selected_feed: PostsList::new(vec![]),
            selected_post: Reader::new("".to_string()),
            selected_post_content: "".to_string(),
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
        // Process all available feed updates without blocking
        while let Ok(fetched) = self.receiver.try_recv() {
            if let Some(existing) = self.feeds.items.iter_mut().find(|f| f.url == fetched.url) {
                feeds::merge_fetched_feed(existing, fetched);
            } else {
                self.feeds.items.push(fetched);
            }

            if self.feeds.items.len() == 1 {
                self.feeds.state.select(Some(0));
            };

            let _ = self.write_state();
        }

        // Poll for events with a short timeout instead of blocking
        if event::poll(Duration::from_millis(250))? {
            match event::read()? {
                Event::Key(key_event) if key_event.kind == KeyEventKind::Press => {
                    self.handle_key_event(key_event)
                }
                _ => Ok(()),
            }
        } else {
            Ok(())
        }
    }

    fn handle_key_event(&mut self, key_event: KeyEvent) -> Result<()> {
        match key_event.code {
            KeyCode::Char('q') => self.exit()?,
            KeyCode::Char('j') | KeyCode::Down => self.select_next(),
            KeyCode::Char('k') | KeyCode::Up => self.select_previous(),
            KeyCode::Char('l') | KeyCode::Right => self.open_selected(),
            KeyCode::Char('h') | KeyCode::Left => self.close_selected(),
            _ => {}
        }
        Ok(())
    }

    fn select_next(&mut self) {
        match self.view {
            View::Feeds => self.feeds.state.select_next(),
            View::Posts => self.selected_feed.state.select_next(),
            View::Post => self.selected_post.scroll_state.scroll_down(),
        }
    }
    fn select_previous(&mut self) {
        match self.view {
            View::Feeds => self.feeds.state.select_previous(),
            View::Posts => self.selected_feed.state.select_previous(),
            View::Post => self.selected_post.scroll_state.scroll_up(),
        }
    }

    fn open_selected(&mut self) {
        match self.view {
            View::Feeds => {
                let id = self.feeds.state.selected();
                self.feeds.state.select(Some(0));

                if let Some(id) = id {
                    self.view = View::Posts;
                    self.selected_feed = PostsList::new(self.feeds.items[id].posts.clone());
                }
            }
            View::Posts => {
                let id = self.selected_feed.state.selected();

                if let Some(id) = id {
                    self.view = View::Post;
                    self.selected_post_content = self.selected_feed.items[id].content.to_string();
                    self.selected_post = Reader::new(self.selected_post_content.clone());
                    self.selected_feed.state.select(Some(0));
                }
            }
            View::Post => (),
        }
    }

    fn close_selected(&mut self) {
        match self.view {
            View::Feeds => (),
            View::Posts => {
                self.view = View::Feeds;
            }
            View::Post => {
                self.view = View::Posts;
            }
        }
    }

    fn exit(&mut self) -> Result<()> {
        self.write_state()?;
        self.exit = true;
        Ok(())
    }
}

impl Widget for &mut App<'_> {
    fn render(self, area: Rect, buf: &mut Buffer) {
        let block = Block::default()
            .borders(Borders::ALL)
            .border_set(border::ROUNDED);

        if self.view == View::Post {
            StatefulWidget::render(
                self.selected_post.clone(),
                area,
                buf,
                &mut self.selected_post.scroll_state,
            );

            return;
        }

        let items: Vec<ListItem> = if self.view == View::Feeds {
            self.feeds
                .items
                .iter()
                .map(|feed| {
                    let title = format!("{}: {}", feed.title, feed.total_unread());
                    ListItem::new(title)
                })
                .collect()
        } else {
            self.selected_feed
                .items
                .iter()
                .map(|post| {
                    let title = post.title.clone().to_string();
                    ListItem::new(title)
                })
                .collect()
        };

        let highlight_style = Style::new()
            .fg(Color::from_str(&self.config.get_invert_text_color()).unwrap())
            .bg(Color::from_str(&self.config.get_accent_color()).unwrap());

        let list = List::new(items)
            .block(block)
            .highlight_style(highlight_style)
            .highlight_spacing(HighlightSpacing::Always);

        let state = if self.view == View::Feeds {
            &mut self.feeds.state
        } else {
            &mut self.selected_feed.state
        };

        StatefulWidget::render(list, area, buf, state);
    }
}
