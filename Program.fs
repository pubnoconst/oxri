open Microsoft.AspNetCore.Builder
open Microsoft.AspNetCore.Diagnostics
open Microsoft.AspNetCore.Http
open Microsoft.AspNetCore.Http.Timeouts
open Microsoft.Extensions.DependencyInjection
open Microsoft.Extensions.Hosting
open Oxpecker
open Oxpecker.ViewEngine
open System
open System.Threading.Tasks

// Keep page routing explicit so each page has a clear, reviewable URL.
let endpoints = [
    route "/" <| Views.Landing.index
    route "/book-i" <| Views.BookI.index
    route "/journals" <| Views.Journals.index
    route "/journal-editor" <| Views.JournalEditor.index
]

let private writeErrorPage (context: HttpContext) (statusCode: int) : Task =
    task {
        if not context.Response.HasStarted then
            context.Response.StatusCode <- statusCode
            context.Response.ContentType <- "text/html; charset=utf-8"
            let body =
                try
                    Views.Error.page statusCode context.TraceIdentifier
                    |> Render.toHtmlDocString
                with _ ->
                    // Keep a plain generic fallback if view or QR rendering fails.
                    $"<!doctype html><html><head><meta charset=\"utf-8\"><title>{statusCode}</title></head><body><main><h1>{statusCode}</h1><p>Something went wrong. Please try again later.</p></main></body></html>"

            do! context.Response.WriteAsync(body)
    }
    :> Task

[<EntryPoint>]
let main args =
    // The default builder loads appsettings.json and environment-specific files
    // such as appsettings.Development.json for the active environment.
    let builder = WebApplication.CreateBuilder(args)
    // Bound application endpoint work so a stalled request can return the same
    // safe timeout page instead of waiting indefinitely.
    let timeoutPolicy =
        RequestTimeoutPolicy(
            Timeout = TimeSpan.FromSeconds(30.0),
            TimeoutStatusCode = StatusCodes.Status504GatewayTimeout,
            WriteTimeoutResponse = RequestDelegate(fun context ->
                writeErrorPage context StatusCodes.Status504GatewayTimeout))
    builder.Services.AddRequestTimeouts(fun options -> options.DefaultPolicy <- timeoutPolicy) |> ignore
    builder.Services.AddRouting().AddOxpecker() |> ignore
    let app = builder.Build()

    // Use the same generic HTML page in every environment. In particular, never
    // enable the developer exception page because it discloses stack traces.
    app.UseExceptionHandler(Action<IApplicationBuilder>(fun errorApp ->
        errorApp.Run(RequestDelegate(fun context ->
            context.Response.Clear()
            writeErrorPage context StatusCodes.Status500InternalServerError))))
    |> ignore

    // Render a consistent page for empty 4xx/5xx responses, including 404 and
    // application-generated timeout statuses such as 408 and 504.
    app.UseStatusCodePages(Func<StatusCodeContext, Task>(fun statusContext ->
        let context = statusContext.HttpContext
        writeErrorPage context context.Response.StatusCode))
    |> ignore

    // Keep error handling outside static files so missing assets also receive
    // the same 404 treatment.
    app.UseStaticFiles() |> ignore
    app.UseRouting()
        .UseRequestTimeouts()
        .UseOxpecker(endpoints)
        .Run(fun context ->
            context.Response.StatusCode <- StatusCodes.Status404NotFound
            Task.CompletedTask)
    app.Run()
    0
