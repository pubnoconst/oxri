use axum::response::IntoResponse;
use maud::html;

use crate::pages::shared;

pub async fn index() -> impl IntoResponse {
    shared::frame(
        "New Journal Entry - RES IMPERII",
        "/page-css/journal-editor.css?v=1",
        html! {
            main class="journal-editor-page" {
                header class="journal-editor-header" {
                    (shared::back_link("/journals", "← Journals"))
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
            script src="/assets/js/journal-editor.js?v=3" {}
        },
    )
}

pub async fn stylesheet() -> impl IntoResponse {
    (
        [(axum::http::header::CONTENT_TYPE, "text/css; charset=utf-8")],
        include_str!("style.css"),
    )
}
