use std::{cell::RefCell, io::Cursor, rc::Rc};

use anyhow::Result;
use html_to_markdown::{markdown, TagHandler};
use serde::{Deserialize, Serialize};
use tokio::sync::mpsc::Sender;

pub type Feeds = Vec<Feed>;

#[derive(Debug, Serialize, Deserialize, Clone)]
pub struct Feed {
    #[serde(skip_serializing)]
    pub title: Box<str>,

    #[serde(rename = "URL")]
    pub url: Box<str>,

    pub posts: Vec<Post>,

    #[serde(skip_serializing)]
    pub id: String,
}

impl Feed {
    pub fn total_unread(&self) -> usize {
        self.posts.iter().filter(|post| !post.read).count()
    }
}

#[derive(Debug, Serialize, Deserialize, Clone)]
pub struct Post {
    pub id: String,
    pub read: bool,

    #[serde(skip_serializing)]
    pub title: Box<str>,

    #[serde(skip_serializing)]
    pub content: Box<str>,

    #[serde(skip_serializing)]
    pub link: Option<Box<str>>,

    #[serde(skip_serializing)]
    pub date: Box<str>,
}

impl Post {
    pub fn mark_as_read(&mut self) {
        self.read = true;
    }

    pub fn mark_as_unread(&mut self) {
        self.read = false;
    }
}

pub async fn fetch_all_feeds(conf: crate::config::Config, sender: Sender<Feed>) -> Result<()> {
    let urls = conf.get_feed_urls();

    let fetches = urls.into_iter().map(|url| {
        let value = sender.clone();
        async move {
            if let Ok(body) = crate::backend::fetch_with_cache(&url).await {
                let data =
                    feed_rs::parser::parse(body.as_bytes()).expect("Failed to parse feed at {url}");

                let title = if let Some(feed_title) = data.title {
                    feed_title.content.into_boxed_str()
                } else {
                    "Untitled Feed".to_owned().into_boxed_str()
                };

                let feed = Feed {
                    title,
                    url: url.into_boxed_str(),
                    posts: data
                        .entries
                        .into_iter()
                        .map(|entry| {
                            let title = if let Some(entry_title) = entry.title {
                                entry_title.content.into_boxed_str()
                            } else {
                                "Untitled Post".to_owned().into_boxed_str()
                            };

                            let content = if let Some(content) = entry.content {
                                if let Some(body) = content.body {
                                    let mut handlers: Vec<TagHandler> = vec![
                                        Rc::new(RefCell::new(markdown::WebpageChromeRemover)),
                                        Rc::new(RefCell::new(markdown::ParagraphHandler)),
                                        Rc::new(RefCell::new(markdown::HeadingHandler)),
                                        Rc::new(RefCell::new(markdown::ListHandler)),
                                        Rc::new(RefCell::new(markdown::TableHandler::new())),
                                        Rc::new(RefCell::new(markdown::StyledTextHandler)),
                                        Rc::new(RefCell::new(markdown::CodeHandler)),
                                    ];

                                    let reader = Cursor::new(body);

                                    html_to_markdown::convert_html_to_markdown(
                                        reader,
                                        &mut handlers,
                                    )
                                    .unwrap_or("No content available".to_owned())
                                    .into_boxed_str()
                                } else {
                                    "No content available".to_owned().into_boxed_str()
                                }
                            } else {
                                "No content available".to_owned().into_boxed_str()
                            };

                            let link = entry
                                .links
                                .first()
                                .map(|link| link.href.clone().into_boxed_str());

                            Post {
                                title,
                                content,
                                link,
                                date: entry
                                    .published
                                    .unwrap_or_default()
                                    .to_string()
                                    .into_boxed_str(),
                                id: entry.id,
                                read: false,
                            }
                        })
                        .collect(),
                    id: data.id,
                };

                value.send(feed).await.ok();
            }
        }
    });

    futures::future::join_all(fetches).await;
    Ok(())
}

pub fn merge_fetched_feed(existing: &mut Feed, fetched: Feed) {
    let read_map = existing
        .posts
        .iter()
        .map(|post| (post.id.clone(), post.read))
        .collect::<std::collections::HashMap<_, _>>();

    existing.title = fetched.title;
    existing.url = fetched.url.clone();

    existing.posts = fetched
        .posts
        .into_iter()
        .map(|mut post| {
            if let Some(&read) = read_map.get(&post.id) {
                post.read = read;
            }
            post
        })
        .collect();
}
