module Views.Landing

open Oxpecker
open Oxpecker.ViewEngine
open Views.Page

let index : EndpointHandler =
    frame "/css/pages/landing.css?v=1" (
        // Keep the landing page to the title and primary destinations. The
        // title is intentionally plain text; only navigation items are links.
        main(class'="landing") {
            nav(class'="landing-nav") {
                h1(class'="landing-title") {
                    "RES·IMPERII"
                }
                ul(class'="landing-books") {
                    li() { a(href="/book-i") { "Book I" } }
                    li() { a(href="/book-ii") { "Book II" } }
                    li() { a(href="/book-iii") { "Book III" } }
                }
                hr(class'="landing-rule")
                a(class'="landing-journals", href="/journals") { "Journals" }
            }
        }
    )
    |> htmlView
