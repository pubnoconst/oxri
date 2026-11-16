mod views;

use axum::{
    Router,
    body::Body,
    http::{StatusCode, header},
    response::Response,
    routing::get,
};
use std::{error::Error, net::SocketAddr};
use tower_http::{
    catch_panic::CatchPanicLayer,
    services::{ServeDir, ServeFile},
};

#[tokio::main]
async fn main() -> Result<(), Box<dyn Error>> {
    let app = Router::new()
        .route("/", get(views::landing))
        .route("/book-i", get(views::book_i))
        .route("/journals", get(views::journals))
        .route("/journal-editor", get(views::journal_editor))
        .nest_service(
            "/css",
            ServeDir::new("wwwroot/css").not_found_service(get(views::not_found)),
        )
        .nest_service(
            "/js",
            ServeDir::new("wwwroot/js").not_found_service(get(views::not_found)),
        )
        .route_service("/favicon.png", ServeFile::new("wwwroot/favicon.png"))
        .fallback(views::not_found)
        .layer(CatchPanicLayer::custom(|_| {
            Response::builder()
                .status(StatusCode::INTERNAL_SERVER_ERROR)
                .header(header::CONTENT_TYPE, "text/html; charset=utf-8")
                .body(Body::from(
                    views::error_page(StatusCode::INTERNAL_SERVER_ERROR).into_string(),
                ))
                .expect("the static generic error response is valid")
        }));

    let address: SocketAddr = std::env::var("ADDR")
        .unwrap_or_else(|_| "127.0.0.1:8080".to_string())
        .parse()?;
    let listener = tokio::net::TcpListener::bind(address).await?;
    println!("Res Imperii Rust experiment listening on {address}");
    axum::serve(listener, app).await?;
    Ok(())
}
