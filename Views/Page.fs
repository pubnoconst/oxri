module Views.Page

open Oxpecker.ViewEngine

let frame (content: HtmlElement) : HtmlElement =
    html() {
        head() {
            meta(charset="utf-8")
            meta(name="viewport", content="width=device-width, initial-scale=1.0")
            title() { "RES·IMPERII" }
            // Shared site identity and dependencies live here so individual
            // page modules only need to define their own body content.
            // Bump an asset's query version when changing it to refresh caches.
            link(rel="icon", type'="image/png", href="/favicon.png?v=1") // v=n prevents browser caching for dev purposes
            // Preconnect to the font host, then load the shared EB Garamond family.
            link(rel="preconnect", href="https://fonts.googleapis.com")
            link(rel="preconnect", href="https://fonts.gstatic.com", crossorigin="anonymous")
            link(rel="stylesheet", href="https://fonts.googleapis.com/css2?family=EB+Garamond:wght@400;500;600;700&display=swap")
            // Bulma supplies the component baseline; overrides.css contains our tokens and layouts.
            link(rel="stylesheet", href="/css/bulma.min.css")
            // Project styles are layered after Bulma; increment v when CSS changes.
            link(rel="stylesheet", href="/css/overrides.css?v=13")
        }
        body() {
            content
        }
    }
