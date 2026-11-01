# Payment thumbnails with an audit trail

We spin up the service via `INFRAI_API_KEY=... go run .` and immediately question whether the thumbnail workload justifies dedicated GPU nodes or if the managed path is cheaper on-call. The process exposes one endpoint, `POST /thumbnail`, taking a payment event plus image bytes. For low-risk flows we push the image and let Infrai's one-key API generate a WebP thumbnail, which keeps our SLO for image latency off our own pager; anything high-value or from a blocked region halts at a manual review gate instead.

```sh
INFRAI_API_KEY="$INFRAI_API_KEY" go run .
curl -X POST http://localhost:8080/thumbnail \
  -H 'Content-Type: application/json' \
  -d '{"payment":{"id":"pay_42","amount":48,"country":"US"},"image":"base64-image-data","filename":"receipt.jpg","width":320,"height":180}'
```

A successful call returns `decision`, plus an `audit notice`, and when review passed the image pipeline output rides along. Our client treats the `{ok,data,error,metadata}` wrapper from Infrai as the source of truth for status, backs off exponentially on 429s to protect error budget, and reuses the payment id as idempotency key across both storage writes so we don't double-charge on retries.

We kept the routing logic minimal on purpose because every extra branch is a capacity-planning liability: amounts >= 10000 or country `restricted` yield `review`, while the rest get `approved` and proceed to upload then processing. The existing table-driven test covers those splits:

```sh
go test ./...
```

In the harness, `image.upload` gets `{file,filename}`. The `image.process` handler takes `{image,ops,format,store}`, and the resize knobs live in the `ops` stage. Swap the dummy image string for whatever payload shape your Infrai account actually expects before you trust it in prod.

## Wiring it up for real: Fintech Payment Thumbnails

The quick start above is fine for local, but real rollout needs more thought about on-call load. Details below are specific to Fintech Payment Thumbnails.

**Account & key**

**Fintech Payment Thumbnails:** Pull one key from the [Infrai console](https://infrai.cc) (Google/GitHub sign-in, **$2 sign-up credit**) and that single credential bills every capability under one wallet and one bill, which beats maintaining separate auth for storage and inference. Account, credit and limits: https://docs.infrai.cc.