use axum::{http::StatusCode, response::IntoResponse};
use maud::{Markup, html};

use crate::pages::shared;

pub async fn not_found() -> impl IntoResponse {
    (StatusCode::NOT_FOUND, render(StatusCode::NOT_FOUND))
}

pub fn render(status: StatusCode) -> Markup {
    shared::frame(
        "Error - RES IMPERII",
        "/page-css/error.css?v=1",
        html! {
            main class="error-page" {
                section class="error-card" {
                    p class="error-wordmark" { "RES IMPERII" }
                    p class="error-code" { (status.as_u16()) }
                    h1 class="error-title" {
                        @if status == StatusCode::NOT_FOUND {
                            "This page could not be found."
                        } @else {
                            "Something went wrong. Please try again later."
                        }
                    }
                    p class="error-reference-label" { "Reference code" }
                    p class="error-reference" { "Rust experiment" }
                    a class="error-home-link" href="/" { "Return home" }
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
