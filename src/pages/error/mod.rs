use std::{panic::AssertUnwindSafe, time::Duration};

use axum::{
    body::Body,
    extract::Request,
    http::{HeaderValue, StatusCode, header},
    middleware::Next,
    response::{IntoResponse, Response},
};
use base64::{Engine, engine::general_purpose::STANDARD};
use maud::{Markup, html};
use qrcode::{QrCode, render::svg};
use uuid::Uuid;

use crate::pages::shared;

/// Return a safe, styled error response for any application-level status.
pub fn response(status: StatusCode) -> Response {
    let body = std::panic::catch_unwind(AssertUnwindSafe(|| render(status).into_string()))
        .unwrap_or_else(|_| fallback_page(status));
    let mut response = Response::new(Body::from(body));
    *response.status_mut() = status;
    response.headers_mut().insert(
        header::CONTENT_TYPE,
        HeaderValue::from_static("text/html; charset=utf-8"),
    );
    response
}

/// Replace any bare 4xx or 5xx response with the common branded error page.
pub async fn status_pages(request: Request, next: Next) -> Response {
    let upstream = next.run(request).await;
    let status = upstream.status();
    if status.is_client_error() || status.is_server_error() {
        response(status)
    } else {
        upstream
    }
}

pub async fn not_found() -> impl IntoResponse {
    StatusCode::NOT_FOUND
}

/// Bound route work so stalled application requests return the same safe page.
pub async fn request_timeout(request: Request, next: Next) -> Response {
    match tokio::time::timeout(Duration::from_secs(30), next.run(request)).await {
        Ok(response) => response,
        Err(_) => StatusCode::GATEWAY_TIMEOUT.into_response(),
    }
}

pub fn render(status: StatusCode) -> Markup {
    let reference = Uuid::new_v4().simple().to_string();
    let qr_data_uri = qr_data_uri(&reference);
    shared::frame(
        "Error - RES IMPERII",
        "/page-css/error.css?v=1",
        html! {
            main class="error-page" {
                section class="error-card" {
                    p class="error-wordmark" { "RES IMPERII" }
                    p class="error-code" { (status.as_u16()) }
                    h1 class="error-title" { (message_for_status(status)) }
                    p class="error-reference-label" { "Reference code" }
                    p class="error-reference" { (reference) }
                    @if let Some(qr_data_uri) = qr_data_uri {
                        div class="error-qr" {
                            img src=(qr_data_uri) alt="QR code containing the reference code";
                            p { "Scan for the reference code" }
                        }
                    }
                    a class="error-home-link" href="/" { "Return home" }
                }
            }
        },
    )
}

fn message_for_status(status: StatusCode) -> &'static str {
    match status {
        StatusCode::FORBIDDEN => "You do not have permission to view this page.",
        StatusCode::NOT_FOUND => "This page could not be found.",
        StatusCode::REQUEST_TIMEOUT => "The request took too long. Please try again.",
        StatusCode::GATEWAY_TIMEOUT => "The service took too long to respond. Please try again.",
        status if status.is_client_error() => "The request could not be completed.",
        _ => "Something went wrong. Please try again later.",
    }
}

fn qr_data_uri(reference: &str) -> Option<String> {
    let code = QrCode::new(reference.as_bytes()).ok()?;
    let svg = code
        .render::<svg::Color>()
        .min_dimensions(160, 160)
        .dark_color(svg::Color("#202124"))
        .light_color(svg::Color("#ffffff"))
        .build();
    Some(format!(
        "data:image/svg+xml;base64,{}",
        STANDARD.encode(svg)
    ))
}

fn fallback_page(status: StatusCode) -> String {
    format!(
        "<!doctype html><html lang=\"en\"><head><meta charset=\"utf-8\"><title>Error - RES IMPERII</title></head><body><main><h1>{}</h1><p>{}</p><a href=\"/\">Return home</a></main></body></html>",
        status.as_u16(),
        message_for_status(status)
    )
}

pub async fn stylesheet() -> impl IntoResponse {
    (
        [(axum::http::header::CONTENT_TYPE, "text/css; charset=utf-8")],
        include_str!("style.css"),
    )
}
