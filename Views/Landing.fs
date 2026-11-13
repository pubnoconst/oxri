module Views.Landing

open Oxpecker
open Oxpecker.ViewEngine
open Views.Page

let index : EndpointHandler =
    frame (
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
