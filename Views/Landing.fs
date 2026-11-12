module Views.Landing

open Oxpecker
open Oxpecker.ViewEngine
open Views.Page

let index : EndpointHandler =
    frame (
        div() {
            h1() { "Hello World" }
        }
    )
    |> htmlView
