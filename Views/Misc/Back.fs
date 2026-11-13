module Views.Misc.Back

open Oxpecker.ViewEngine

/// A reusable link back to the landing page, with a compact circular chevron.
/// Keep the visible label in markup; CSS supplies the icon without an extra library.
let view =
    a(class'="back-link", href="/") {
        span(class'="back-icon") { }
        span(class'="back-label") { "Back" }
    }
