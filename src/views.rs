use axum::{http::StatusCode, response::IntoResponse};
use maud::{DOCTYPE, Markup, html};

fn page(title: &str, stylesheet: &str, content: Markup) -> Markup {
    html! {
        (DOCTYPE)
        html lang="en" {
            head {
                meta charset="utf-8";
                meta name="viewport" content="width=device-width, initial-scale=1";
                title { (title) }
                link rel="icon" type="image/png" href="/favicon.png?v=1";
                link rel="preconnect" href="https://fonts.googleapis.com";
                link rel="preconnect" href="https://fonts.gstatic.com" crossorigin="anonymous";
                link rel="stylesheet" href="https://fonts.googleapis.com/css2?family=EB+Garamond:wght@400;500;600;700&display=swap";
                link rel="stylesheet" href="/css/bulma.min.css";
                link rel="stylesheet" href="/css/overrides.css?v=21";
                link rel="stylesheet" href=(stylesheet);
            }
            body {
                (content)
                script src="https://unpkg.com/htmx.org@2.0.4" {}
                script src="/js/theme-toggle.js?v=1" {}
                script src="/js/navigation-feedback.js?v=1" {}
            }
        }
    }
}

pub async fn landing() -> impl IntoResponse {
    page(
        "RES IMPERII",
        "/css/pages/landing.css?v=1",
        html! {
            main class="landing" hx-boost="true" {
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

pub async fn book_i() -> impl IntoResponse {
    page(
        "Book I - RES IMPERII",
        "/css/pages/book-i.css?v=1",
        html! {
            main class="article-page" {
                article class="book-article" {
                    a class="back-link" href="/" { "← Back" }
                    header class="article-header" {
                        h1 { "Book I" }
                        p class="article-deck" { "On cause." }
                    }
                    section {
                        p { "Cause and effect follow each other necessarily. The difference between the two is not ontic, but epistemic. What is a cause is an effect of the cause prior; inversely, what is an effect, is a cause of the effect ex post. The process of effectuation given a prior set of causes is Causal Succession." }
                    }
                    p class="rust-note" { "This route is an initial Rust rendering experiment; canonical article content remains on the main branch." }
                }
            }
        },
    )
}

pub async fn journals() -> impl IntoResponse {
    page(
        "Journals - RES IMPERII",
        "/css/pages/journals.css?v=1",
        html! {
            main class="journals-page" {
                header class="journals-header" {
                    a class="back-link" href="/" { "← Back" }
                    h1 { "Journals" }
                }
                p class="journals-empty" { "No entries yet." }
                a class="new-entry-link" href="/journal-editor" { "Add New Entry" }
            }
        },
    )
}

pub async fn journal_editor() -> impl IntoResponse {
    page(
        "New Journal Entry - RES IMPERII",
        "/css/pages/journal-editor.css?v=1",
        html! {
            main class="journal-editor-page" {
                header class="journal-editor-header" {
                    a class="back-link" href="/journals" { "← Journals" }
                    div class="editor-heading" {
                        div {
                            p class="editor-eyebrow" { "JOURNALS / NEW ENTRY" }
                            h1 { "A new entry" }
                        }
                        p class="editor-notice" { "Draft preview - not saved" }
                    }
                }
                div class="journal-editor-grid" {
                    section class="editor-pane editor-input-card" {
                        div class="editor-section-heading" { h2 { "Compose" } span { "01" } }
                        div class="editor-field" {
                            label for="journal-media-link" { "Media link" }
                            input id="journal-media-link" type="url" placeholder="https://..." autocomplete="url";
                            p class="field-hint" { "Paste a video, audio, or image link to show it in the preview." }
                        }
                        div class="editor-field" {
                            label for="journal-tags" { "Tags" }
                            input id="journal-tags" type="text" placeholder="history, notes, travel";
                            p class="field-hint" { "Separate tags with commas." }
                        }
                        div class="editor-field editor-content-field" {
                            label for="journal-content" { "Entry content" }
                            textarea id="journal-content" rows="16" { "# A journal entry\n\nWrite here and see it take shape in the preview.\n\n## A section\n\nAdd **emphasis**, lists, links, or `inline code`.\n\n- First thought\n- Another thought" }
                        }
                    }
                    section class="editor-pane editor-preview-pane" {
                        div class="editor-section-heading" { h2 { "Live preview" } span { "02" } }
                        article id="journal-preview" class="journal-preview journal-preview-card" {
                            div id="journal-preview-media" class="journal-preview-media" title="Media preview" {}
                            div class="journal-preview-body" {
                                div id="journal-preview-tags" class="journal-preview-tags" {}
                                div id="journal-rendered-content" class="journal-rendered-content" {}
                            }
                        }
                    }
                }
                div class="journal-editor-actions" {
                    p class="publish-note" { "Publishing will be available soon." }
                    button type="button" disabled { "Publish entry" }
                }
            }
            script src="https://cdn.jsdelivr.net/npm/markdown-it@15.0.3/dist/browser/markdown-it.umd.min.js" {}
            script src="https://cdn.jsdelivr.net/npm/dompurify@3.4.17/dist/purify.min.js" {}
            script src="/js/journal-editor.js?v=3" {}
        },
    )
}

pub async fn not_found() -> impl IntoResponse {
    (StatusCode::NOT_FOUND, error_page(StatusCode::NOT_FOUND))
}

pub(crate) fn error_page(status: StatusCode) -> Markup {
    page(
        "Error - RES IMPERII",
        "/css/pages/error.css?v=1",
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
