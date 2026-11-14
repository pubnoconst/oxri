module Views.JournalEditor

open Oxpecker
open Oxpecker.ViewEngine
open Views.Misc
open Views.Page

let index : EndpointHandler =
    frame "/css/pages/journal-editor.css?v=1" (
        // This page is registered only in Development; treat it as a visual
        // prototype until an authenticated persistence flow exists.
        main(class'="journal-editor-page") {
            header(class'="journal-editor-header") {
                Back.view
                div(class'="editor-heading") {
                    div() {
                        p(class'="editor-eyebrow") { "JOURNALS / NEW ENTRY" }
                        h1() { "A new entry" }
                    }
                    p(class'="editor-notice") { "Draft preview - not saved" }
                }
            }

            div(class'="journal-editor-grid") {
                // Keep all draft inputs together: media URL, comma-separated
                // tags, and the Markdown source that drives the live preview.
                section(class'="editor-pane editor-input-card") {
                    div(class'="editor-section-heading") {
                        h2() { "Compose" }
                        span() { "01" }
                    }
                    div(class'="editor-field") {
                        label(for'="journal-media-link") { "Media link" }
                        input(id="journal-media-link", type'="url", placeholder="https://...", autocomplete="url")
                        p(class'="field-hint") { "Paste a video, audio, or image link to show it in the preview." }
                    }
                    div(class'="editor-field") {
                        label(for'="journal-tags") { "Tags" }
                        input(id="journal-tags", type'="text", placeholder="history, notes, travel")
                        p(class'="field-hint") { "Separate tags with commas." }
                    }
                    div(class'="editor-field editor-content-field") {
                        label(for'="journal-content") { "Entry content" }
                        textarea(id="journal-content", rows=16) {
                            "# A journal entry\n\nWrite here and see it take shape in the preview.\n\n## A section\n\nAdd **emphasis**, lists, links, or `inline code`.\n\n- First thought\n- Another thought"
                        }
                    }
                }

                // The preview mirrors the published card: centered media first,
                // followed by tags and the rendered entry body.
                section(class'="editor-pane editor-preview-pane") {
                    div(class'="editor-section-heading") {
                        h2() { "Live preview" }
                        span() { "02" }
                    }
                    article(class'="journal-preview journal-preview-card", id="journal-preview") {
                        div(class'="journal-preview-media", id="journal-preview-media", title="Media preview") { }
                        div(class'="journal-preview-body") {
                            div(class'="journal-preview-tags", id="journal-preview-tags") { }
                            div(class'="journal-rendered-content", id="journal-rendered-content") { }
                        }
                    }
                }
            }

            // Publishing stays unavailable while this prototype has no save
            // endpoint or author authentication.
            div(class'="journal-editor-actions") {
                p(class'="publish-note") { "Publishing will be available soon." }
                button(type'="button", disabled=true) { "Publish entry" }
            }

            // Load pinned browser libraries only on this prototype route. The
            // local script disables raw Markdown HTML, sanitizes rendered output,
            // and keeps all draft content in the browser without sending it.
            script(src="https://cdn.jsdelivr.net/npm/markdown-it@15.0.3/dist/browser/markdown-it.umd.min.js") { }
            script(src="https://cdn.jsdelivr.net/npm/dompurify@3.4.17/dist/purify.min.js") { }
            script(src="/js/journal-editor.js?v=3") { }
        }
    )
    |> htmlView
