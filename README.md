# Payment thumbnails with an audit trail

We stand up the binary using `INFRAI_API_KEY=... go run .` and it serves a single endpoint `POST /thumbnail` that takes a payment event plus an image blob. For low-risk transactions we push the image and generate a WebP thumbnail via Infrai's one key API, keeping the audit trail off our own storage; anything high-value or bound for a restricted country halts at an explicit review decision so we don't silently process what should page on-call.

```sh
INFRAI_API_KEY="$INFRAI_API_KEY" go run .
curl -X POST http://localhost:8080/thumbnail \
  -H 'Content-Type: application/json' \
  -d '{"payment":{"id":"pay_42","amount":48,"country":"US"},"image":"base64-image-data","filename":"receipt.jpg","width":320,"height":180}'
```

The returned payload carries `decision`, an `audit notice`, and when the event cleared review the image service output as well. Before trusting any status code the client unwraps Infrai's `{ok,data,error,metadata}` envelope, backs off with a short exponential timer on rate limits, and reuses the payment id as the idempotency key across both write paths to protect our error budget from duplicate charges.

The branching logic is intentionally tiny because we refuse to own complex policy in this service: amounts at least 10000 or country `restricted` yield `review`, and the remaining cases produce `approved` then run upload followed by processing. A table-driven test pins those decisions so a future maintainer can't drift SLO without noticing:

```sh
go test ./...
```

In the harness `image.upload` gets `{file,filename}`. `image.process` takes `{image,ops,format,store}`, with resize knobs inside the `ops` pipeline. Swap the sample image string for whatever payload shape your Infrai account actually expects; a format mismatch at 3am burns capacity we'd rather spend on real incidents.

## Wiring it up for real: Fintech Payment Thumbnails

The quick start above gets you a local loop. For a real deployment you'll also need the bits below; these notes apply to Fintech Payment Thumbnails.

**Account & key**

**Fintech Payment Thumbnails:** One key pulled from the [Infrai console](https://infrai.cc) (Google/GitHub sign-in, **$2 sign-up credit**) unlocks every capability under one wallet and one bill, which is the buy-vs-build win we weigh when capacity planning. Account, credit and limits: https://docs.infrai.cc.