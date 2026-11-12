open Microsoft.AspNetCore.Builder
open Microsoft.Extensions.DependencyInjection
open Oxpecker

let endpoints = [
    route "/" <| Views.Home.index
]

[<EntryPoint>]
let main args =
    let builder = WebApplication.CreateBuilder(args)
    builder.Services.AddRouting().AddOxpecker() |> ignore
    let app = builder.Build()
    app.UseStaticFiles() 
    app.UseRouting()
        .Use(Default.exceptionMiddleware)
        .UseOxpecker(endpoints)
        .Run Default.notFoundHandler
    app.Run()
    0
