open Microsoft.AspNetCore.Builder
open Microsoft.Extensions.DependencyInjection
open Oxpecker

let endpoints = [
    route "/" <| Views.Landing.index
    route "/book-i" <| Views.BookI.index
]

[<EntryPoint>]
let main args =
    let builder = WebApplication.CreateBuilder(args)
    builder.Services.AddRouting().AddOxpecker() |> ignore
    let app = builder.Build()
    app.UseStaticFiles() |> ignore
    app.UseRouting()
        .Use(Default.exceptionMiddleware)
        .UseOxpecker(endpoints)
        .Run Default.notFoundHandler
    app.Run()
    0
