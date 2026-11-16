use axum::response::IntoResponse;
use maud::html;

use crate::pages::shared;

pub async fn index() -> impl IntoResponse {
    shared::frame(
        "Journals - RES IMPERII",
        "/page-css/journals.css?v=1",
        html! {
            main class="journals-page" {
                header class="journals-header" {
                    (shared::back_link("/", "← Back"))
                    h1 { "Journals" }
                }
                p class="journals-empty" { "No entries yet." }
                a class="new-entry-link" href="/journal-editor" { "Add New Entry" }
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
