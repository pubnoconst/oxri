use maud::{DOCTYPE, Markup, html};

/// Render the shared document shell around a page's route-specific content.
pub fn frame(title: &str, stylesheet: &str, content: Markup) -> Markup {
    html! {
        (DOCTYPE)
        html lang="en" {
            head {
                meta charset="utf-8";
                meta name="viewport" content="width=device-width, initial-scale=1";
                title { (title) }
                link rel="icon" type="image/png" href="/assets/favicon.png?v=1";
                link rel="preconnect" href="https://fonts.googleapis.com";
                link rel="preconnect" href="https://fonts.gstatic.com" crossorigin="anonymous";
                link rel="stylesheet" href="https://fonts.googleapis.com/css2?family=EB+Garamond:wght@400;500;600;700&display=swap";
                link rel="stylesheet" href="/assets/css/bulma.min.css";
                link rel="stylesheet" href="/assets/css/overrides.css?v=21";
                link rel="stylesheet" href=(stylesheet);
            }
            body {
                (content)
                script src="https://unpkg.com/htmx.org@2.0.4" {}
                script src="/assets/js/theme-toggle.js?v=1" {}
                script src="/assets/js/navigation-feedback.js?v=1" {}
            }
        }
    }
}

/// Render the shared circular-chevron navigation link used by article pages.
pub fn back_link(href: &str, label: &str) -> Markup {
    html! {
        a class="back-link" href=(href) { (label) }
    }
}
