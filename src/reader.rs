use ratatui::{
    prelude::Widget,
    text::Text,
    widgets::{Paragraph, StatefulWidget},
};

#[derive(Debug, Clone, Copy)]
pub struct ScrollState {
    position: usize,
    total: usize,
    pub view_size: usize,
}

#[derive(Debug, Clone)]
pub struct Reader<'a> {
    pub scroll_state: ScrollState,
    pub content: Text<'a>,
}

impl ScrollState {
    pub fn new(total: usize) -> Self {
        Self {
            position: 0,
            total,
            view_size: 1,
        }
    }

    pub fn scroll_down(&mut self) {
        if self.position < self.total {
            self.position = self.position.saturating_add(1);
        }
    }

    pub fn scroll_up(&mut self) {
        self.position = self.position.saturating_sub(1);
    }
}

impl<'a> Reader<'a> {
    pub fn new(raw: String) -> Self {
        // please never let me touch this language again
        let raw_ref = Box::leak(Box::new(raw));
        let rendered = tui_markdown::from_str(raw_ref);

        Reader {
            scroll_state: ScrollState::new(rendered.height()),
            content: rendered,
        }
    }
}

impl StatefulWidget for Reader<'_> {
    type State = ScrollState;

    fn render(
        self,
        area: ratatui::layout::Rect,
        buf: &mut ratatui::buffer::Buffer,
        state: &mut Self::State,
    ) {
        state.view_size = area.height as usize;

        let position = state
            .position
            .min(self.content.height().saturating_sub(state.view_size))
            as u16;

        Paragraph::new(self.content.clone())
            .scroll((position, 0))
            .wrap(ratatui::widgets::Wrap { trim: false })
            .render(area, buf);
    }
}
