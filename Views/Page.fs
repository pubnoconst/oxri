module Views.Page

open Oxpecker.ViewEngine

let frame (content: HtmlElement) : HtmlElement =
    html() {
        head() {
            meta(charset="utf-8")
            meta(name="viewport", content="width=device-width, initial-scale=1.0")
            title() { "RES·IMPERII" }
            // Bump the query version when this static icon changes to refresh browser caches.
            link(rel="icon", type'="image/png", href="/favicon.png?v=1") // v=n prevents browser caching for dev purposes
            // Preconnect to the font host, then load the shared EB Garamond family.
            link(rel="preconnect", href="https://fonts.googleapis.com")
            link(rel="preconnect", href="https://fonts.gstatic.com", crossorigin="anonymous")
            link(rel="stylesheet", href="https://fonts.googleapis.com/css2?family=EB+Garamond:wght@400;500;600;700&display=swap")
            // Bulma supplies the component baseline; overrides.css contains our tokens and layouts.
            link(rel="stylesheet", href="/css/bulma.min.css")
            link(rel="stylesheet", href="/css/overrides.css?v=9")
        }
        body() {
            content
        }
    }
