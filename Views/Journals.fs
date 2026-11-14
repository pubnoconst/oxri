module Views.Journals

open Oxpecker
open Oxpecker.ViewEngine
open Views.Misc
open Views.Page

let index : EndpointHandler =
    frame (
        // Development-only catalog placeholder. The route is intentionally an
        // empty state until authentication and journal storage are designed.
        main(class'="journals-page") {
            header(class'="journals-header") {
                Back.view
                h1() { "Journals" }
            }
            p(class'="journals-empty") { "No entries yet." }
            // The linked editor is also Development-only and does not save.
            a(class'="new-entry-link", href="/journal-editor") { "Add New Entry" }
        }
    )
    |> htmlView
