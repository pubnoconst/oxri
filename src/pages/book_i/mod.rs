use axum::response::IntoResponse;
use maud::{PreEscaped, html};
use pulldown_cmark::{Event, Parser, html as markdown_html};

use crate::pages::shared;

const BOOK_I_MARKDOWN: &str = include_str!("../../../rawcontent/book-i.md");

pub async fn index() -> impl IntoResponse {
    let mut rendered_markdown = String::with_capacity(BOOK_I_MARKDOWN.len() * 2);
    let markdown_events = Parser::new(BOOK_I_MARKDOWN)
        // Preserve Markdown formatting while excluding authored raw HTML.
        .filter(|event| !matches!(event, Event::Html(_) | Event::InlineHtml(_)));
    markdown_html::push_html(&mut rendered_markdown, markdown_events);

    shared::frame(
        "Book I - RES IMPERII",
        "/page-css/book-i.css?v=2",
        html! {
            main class="article-page" {
                article class="book-article" {
                    (shared::back_link("/", "Back"))
                    section class="book-content" {
                        (PreEscaped(rendered_markdown))
                    }
                    (shared::back_link("/", "Back"))
                }
            }
        },
    )
}

pub async fn stylesheet() -> impl IntoResponse {
    (
        [(axum::http::header::CONTENT_TYPE, "text/css; charset=utf-8")],
        include_str!("style.css"),
    )
}
