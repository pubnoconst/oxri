open Microsoft.AspNetCore.Builder
open Microsoft.Extensions.DependencyInjection
open Oxpecker

// Keep page routing explicit so each page has a clear, reviewable URL.
let endpoints = [
    route "/" <| Views.Landing.index
    route "/book-i" <| Views.BookI.index
]

[<EntryPoint>]
let main args =
    // The default builder loads appsettings.json and environment-specific files
    // such as appsettings.Development.json for the active environment.
    let builder = WebApplication.CreateBuilder(args)
    builder.Services.AddRouting().AddOxpecker() |> ignore
    let app = builder.Build()
    // Serve public CSS and other assets from wwwroot before routing page requests.
    app.UseStaticFiles() |> ignore
    app.UseRouting()
        .Use(Default.exceptionMiddleware)
        .UseOxpecker(endpoints)
        .Run Default.notFoundHandler
    app.Run()
    0
