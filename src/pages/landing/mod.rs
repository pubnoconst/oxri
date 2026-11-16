use axum::response::IntoResponse;
use maud::html;

use crate::pages::shared;

pub async fn index() -> impl IntoResponse {
    shared::frame(
        "RES IMPERII",
        "/page-css/landing.css?v=1",
        html! {
            main class="landing" {
                nav class="landing-nav" {
                    h1 class="landing-title" { "RES·IMPERII" }
                    ul class="landing-books" {
                        li { a href="/book-i" { "Book I" } }
                        li { a href="/book-ii" { "Book II" } }
                        li { a href="/book-iii" { "Book III" } }
                    }
                    hr class="landing-rule";
                    a class="landing-journals" href="/journals" { "Journals" }
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
