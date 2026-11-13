module Views.Page

open Oxpecker.ViewEngine

let frame (content: HtmlElement) : HtmlElement =
    html() {
        head() {
            meta(charset="utf-8")
            meta(name="viewport", content="width=device-width, initial-scale=1.0")
            title() { "RES·IMPERII" }
            link(rel="icon", type'="image/png", href="/favicon.png")
            link(rel="preconnect", href="https://fonts.googleapis.com")
            link(rel="preconnect", href="https://fonts.gstatic.com", crossorigin="anonymous")
            link(rel="stylesheet", href="https://fonts.googleapis.com/css2?family=EB+Garamond:wght@400;500;600;700&display=swap")
            link(rel="stylesheet", href="/css/bulma.min.css")
            link(rel="stylesheet", href="/css/overrides.css")
        }
        body() {
            content
        }
    }
