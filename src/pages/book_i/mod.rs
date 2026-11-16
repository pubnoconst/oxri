use axum::response::IntoResponse;
use maud::html;

use crate::pages::shared;

pub async fn index() -> impl IntoResponse {
    shared::frame(
        "Book I - RES IMPERII",
        "/page-css/book-i.css?v=1",
        html! {
            main class="article-page" {
                article class="book-article" {
                    (shared::back_link("/", "← Back"))
                    header class="article-header" {
                        h1 { "Book I" }
                        p class="article-deck" { "On cause." }
                    }
                    section {
                        p { "Cause and effect follow each other necessarily. The difference between the two is not ontic, but epistemic. What is a cause is an effect of the cause prior; inversely, what is an effect, is a cause of the effect ex post. The process of effectuation given a prior set of causes is Causal Succession." }
                    }
                    p class="rust-note" { "This route is an initial Rust rendering experiment; canonical article content remains on the main branch." }
                    (shared::back_link("/", "← Back"))
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
