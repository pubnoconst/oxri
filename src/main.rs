mod pages;

use axum::{
    Router,
    body::Body,
    http::{StatusCode, header},
    response::Response,
    routing::get,
};
use std::{error::Error, net::SocketAddr};
use tower_http::{catch_panic::CatchPanicLayer, services::ServeDir};

#[tokio::main]
async fn main() -> Result<(), Box<dyn Error>> {
    let app = Router::new()
        .route("/", get(pages::landing::index))
        .route("/page-css/landing.css", get(pages::landing::stylesheet))
        .route("/book-i", get(pages::book_i::index))
        .route("/page-css/book-i.css", get(pages::book_i::stylesheet))
        .route("/journals", get(pages::journals::index))
        .route("/page-css/journals.css", get(pages::journals::stylesheet))
        .route("/journal-editor", get(pages::journal_editor::index))
        .route(
            "/page-css/journal-editor.css",
            get(pages::journal_editor::stylesheet),
        )
        .route("/page-css/error.css", get(pages::error::stylesheet))
        .nest_service(
            "/assets",
            ServeDir::new("assets").not_found_service(get(pages::error::not_found)),
        )
        .fallback(pages::error::not_found)
        .layer(CatchPanicLayer::custom(|_| {
            Response::builder()
                .status(StatusCode::INTERNAL_SERVER_ERROR)
                .header(header::CONTENT_TYPE, "text/html; charset=utf-8")
                .body(Body::from(
                    pages::error::render(StatusCode::INTERNAL_SERVER_ERROR).into_string(),
                ))
                .expect("the static generic error response is valid")
        }));

    let address: SocketAddr = std::env::var("ADDR")
        .unwrap_or_else(|_| "127.0.0.1:8080".to_string())
        .parse()?;
    let listener = tokio::net::TcpListener::bind(address).await?;
    println!("Res Imperii Rust application listening on {address}");
    axum::serve(listener, app).await?;
    Ok(())
}
