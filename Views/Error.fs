module Views.Error

open System
open Oxpecker.ViewEngine
open QRCoder
open Views.Page

// Keep the public message generic. The HTTP status and opaque request reference
// help the visitor identify the failure without exposing exception details.
let private messageForStatus (statusCode: int) =
    match statusCode with
    | 404 -> "This page could not be found."
    | 408 -> "The request took too long. Please try again."
    | 504 -> "The service took too long to respond. Please try again."
    | _ -> "Something went wrong. Please try again later."

// Encode only the reference code in the QR. A future logging/reporting feature
// can use the same value to locate an incident without putting private data in it.
let private qrDataUri (referenceCode: string) =
    try
        use generator = new QRCodeGenerator()
        use qrData = generator.CreateQrCode(referenceCode, QRCodeGenerator.ECCLevel.M)
        use qrCode = new PngByteQRCode(qrData)
        "data:image/png;base64," + Convert.ToBase64String(qrCode.GetGraphic(8))
    with _ ->
        // A QR rendering failure must never stop the error page itself rendering.
        ""

let page (statusCode: int) (referenceCode: string) : HtmlElement =
    let qr = qrDataUri referenceCode

    frame "/css/pages/error.css?v=1" (
        main(class'="error-page") {
            section(class'="error-card") {
                p(class'="error-wordmark") { "RES IMPERII" }
                p(class'="error-code") { string statusCode }
                h1(class'="error-title") { messageForStatus statusCode }
                p(class'="error-reference-label") { "Reference code" }
                p(class'="error-reference") { referenceCode }
                if not (String.IsNullOrWhiteSpace qr) then
                    div(class'="error-qr") {
                        img(src=qr, alt="QR code containing the reference code")
                        p() { "Scan for the reference code" }
                    }
                a(class'="error-home-link", href="/") { "Return home" }
            }
        }
    )
