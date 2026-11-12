module Views.Page

open Oxpecker.ViewEngine

let frame (content: HtmlElement) : HtmlElement =
    html() {
        head() {
            meta(charset="utf-8")
            meta(name="viewport", content="width=device-width, initial-scale=1.0")
            title() { "RES·IMPERII" }
            link(rel="stylesheet", href="/css/bulma.min.css")
            link(rel="stylesheet", href="/css/overrides.css")
        }
        body() {
            content
        }
    }
