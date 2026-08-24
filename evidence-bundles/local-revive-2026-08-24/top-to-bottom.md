# Top-to-bottom native path 2026-08-24

Local Windows. Not Fuji.

## Spine

UI / `POST /api/orders` → order-router `POST /orders` → VeilVM `CommitOrder`.

`GET /api/markets` merges VeilVM native rows (`GET :9098/markets`) ahead of Polymarket catalog.

Polygon/Kalshi: catalog only. `nativeNetwork=polygon` returns HTTP 501 `POLYGON_PASSTHROUGH_CATALOG_ONLY`.

## E2E (`npm run e2e:top-to-bottom`)

PASS:

1. Router health
2. `POST /native/create-market`
3. `GET /markets` includes the row
4. `POST /orders` veil_native → `veilTxHash` `0x68ae50c8…225073`
5. Polygon passthrough 501

## How to run

```
powershell -NoProfile -ExecutionPolicy Bypass -File scripts\run-local-stack.ps1
node scripts\top-to-bottom.mjs
```

Frontend (dev): `veil-frontend` with `VEIL_ORDER_API_BASE=http://127.0.0.1:9098`. Native markets show **Trade on VeilVM**.
