mod pages;

use axum::{Router, http::StatusCode, middleware, response::IntoResponse, routing::get};
use std::{error::Error, net::SocketAddr};
use tower_http::{catch_panic::CatchPanicLayer, services::ServeDir};

pub fn app() -> Router {
    Router::new()
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
            StatusCode::INTERNAL_SERVER_ERROR.into_response()
        }))
        .layer(middleware::from_fn(pages::error::request_timeout))
        .layer(middleware::from_fn(pages::error::status_pages))
}

#[cfg(test)]
mod tests {
    use super::*;
    use axum::body::{Body, to_bytes};
    use axum::http::Request;
    use tower::ServiceExt;

    #[tokio::test]
    async fn test_not_found_status_and_body() {
        let app = app();
        let response = app
            .oneshot(
                Request::builder()
                    .uri("/nonexistent")
                    .body(Body::empty())
                    .unwrap(),
            )
            .await
            .unwrap();
        let status = response.status();
        let body = to_bytes(response.into_body(), usize::MAX).await.unwrap();
        let body_str = String::from_utf8_lossy(&body);
        assert_eq!(status, StatusCode::NOT_FOUND);
        assert!(body_str.contains("RES IMPERII"));
        assert!(body_str.contains("404"));
    }

    #[tokio::test]
    async fn test_landing_page_navigation_not_boosted() {
        let app = app();
        let response = app
            .oneshot(Request::builder().uri("/").body(Body::empty()).unwrap())
            .await
            .unwrap();
        let status = response.status();
        let body = to_bytes(response.into_body(), usize::MAX).await.unwrap();
        let body_str = String::from_utf8_lossy(&body);
        assert_eq!(status, StatusCode::OK);
        assert!(
            !body_str.contains("hx-boost"),
            "landing page should use native navigation for same-origin routes"
        );
        assert!(body_str.contains("Book I"));
        assert!(body_str.contains("Book II"));
        assert!(body_str.contains("Journals"));
        assert!(body_str.contains("htmx.org"));
        assert!(body_str.contains("alpinejs"));
    }

    #[tokio::test]
    async fn test_book_ii_not_found_renders_error_page() {
        let app = app();
        let response = app
            .oneshot(
                Request::builder()
                    .uri("/book-ii")
                    .body(Body::empty())
                    .unwrap(),
            )
            .await
            .unwrap();
        let status = response.status();
        let body = to_bytes(response.into_body(), usize::MAX).await.unwrap();
        let body_str = String::from_utf8_lossy(&body);
        assert_eq!(status, StatusCode::NOT_FOUND);
        assert!(body_str.contains("RES IMPERII"));
        assert!(body_str.contains("404"));
        assert!(body_str.contains("This page could not be found."));
        assert!(body_str.contains("/page-css/error.css"));
    }

    #[tokio::test]
    async fn test_book_i_article_renders_and_links_stylesheet() {
        let app = app();
        let response = app
            .oneshot(
                Request::builder()
                    .uri("/book-i")
                    .body(Body::empty())
                    .unwrap(),
            )
            .await
            .unwrap();
        let status = response.status();
        let body = to_bytes(response.into_body(), usize::MAX).await.unwrap();
        let body_str = String::from_utf8_lossy(&body);
        assert_eq!(status, StatusCode::OK);
        assert!(body_str.contains("book-article"));
        assert!(body_str.contains("book-content"));
        assert!(body_str.contains("/page-css/book-i.css"));
    }

    #[tokio::test]
    async fn test_journals_renders_and_links_stylesheet() {
        let app = app();
        let response = app
            .oneshot(
                Request::builder()
                    .uri("/journals")
                    .body(Body::empty())
                    .unwrap(),
            )
            .await
            .unwrap();
        let status = response.status();
        let body = to_bytes(response.into_body(), usize::MAX).await.unwrap();
        let body_str = String::from_utf8_lossy(&body);
        assert_eq!(status, StatusCode::OK);
        assert!(body_str.contains("journals-page"));
        assert!(body_str.contains("/page-css/journals.css"));
    }

    #[tokio::test]
    async fn test_panic_handling() {
        let test_router = Router::new()
            .route(
                "/panic",
                get(|| async {
                    panic!("test panic");
                }),
            )
            .layer(CatchPanicLayer::custom(|_| {
                StatusCode::INTERNAL_SERVER_ERROR.into_response()
            }))
            .layer(middleware::from_fn(pages::error::request_timeout))
            .layer(middleware::from_fn(pages::error::status_pages));

        let response = test_router
            .oneshot(
                Request::builder()
                    .uri("/panic")
                    .body(Body::empty())
                    .unwrap(),
            )
            .await
            .unwrap();
        let status = response.status();
        let body = to_bytes(response.into_body(), usize::MAX).await.unwrap();
        let body_str = String::from_utf8_lossy(&body);
        println!("Panic response status: {status}");
        assert_eq!(status, StatusCode::INTERNAL_SERVER_ERROR);
        assert!(body_str.contains("500"));
        assert!(body_str.contains("RES IMPERII"));
    }

    #[tokio::test]
    async fn test_timeout_handling() {
        let test_router = Router::new()
            .route(
                "/slow",
                get(|| async {
                    tokio::time::sleep(std::time::Duration::from_millis(100)).await;
                    "ok"
                }),
            )
            .layer(middleware::from_fn(
                |req, next: middleware::Next| async move {
                    match tokio::time::timeout(std::time::Duration::from_millis(10), next.run(req))
                        .await
                    {
                        Ok(res) => res,
                        Err(_) => StatusCode::GATEWAY_TIMEOUT.into_response(),
                    }
                },
            ))
            .layer(middleware::from_fn(pages::error::status_pages));

        let response = test_router
            .oneshot(Request::builder().uri("/slow").body(Body::empty()).unwrap())
            .await
            .unwrap();
        let status = response.status();
        let body = to_bytes(response.into_body(), usize::MAX).await.unwrap();
        let body_str = String::from_utf8_lossy(&body);
        println!("Timeout response status: {status}");
        assert_eq!(status, StatusCode::GATEWAY_TIMEOUT);
        assert!(body_str.contains("504"));
        assert!(body_str.contains("RES IMPERII"));
    }
}

#[tokio::main]
async fn main() -> Result<(), Box<dyn Error>> {
    let app = app();

    let address: SocketAddr = std::env::var("ADDR")
        .unwrap_or_else(|_| "127.0.0.1:8080".to_string())
        .parse()?;
    let listener = tokio::net::TcpListener::bind(address).await?;
    println!("Res Imperii Rust application listening on {address}");
    axum::serve(listener, app).await?;
    Ok(())
}
