#!/usr/bin/env node
/**
 * Spine check: router /markets + /orders → VeilVM CommitOrder.
 * Does not start the node. Requires local stack (9660 + 9098).
 */
const ROUTER = process.env.ORDER_ROUTER_URL || "http://127.0.0.1:9098";
const SECRET = process.env.ORDER_ROUTER_RELAY_SECRET || "local-dev-secret";

async function call(path, { method = "GET", body } = {}) {
  const headers = { "x-relay-secret": SECRET };
  if (body) headers["content-type"] = "application/json";
  const res = await fetch(`${ROUTER}${path}`, {
    method,
    headers,
    body: body ? JSON.stringify(body) : undefined,
  });
  const json = await res.json().catch(() => ({}));
  if (!res.ok) {
    throw new Error(`${method} ${path} ${res.status}: ${JSON.stringify(json)}`);
  }
  return json;
}

const health = await call("/health");
if (!health.ok) throw new Error(`router unhealthy: ${JSON.stringify(health)}`);
console.log(`1) router ok chain=${health.chainId}`);

const created = await call("/native/create-market", {
  method: "POST",
  body: { question: "top-to-bottom native", outcomes: 2, creatorBond: 1 },
});
if (!created.accepted || !created.marketId) throw new Error(`create-market: ${JSON.stringify(created)}`);
console.log(`2) market ${created.marketId} tx=${created.veilTxHash}`);

const listed = await call("/markets");
const hit = (listed.markets || []).find((m) => m.marketId === created.marketId);
if (!hit) throw new Error(`market missing from GET /markets`);
console.log(`3) GET /markets has native row`);

const order = await call("/orders", {
  method: "POST",
  body: {
    marketId: created.marketId,
    side: "buy",
    outcome: "yes",
    amountUsd: 25,
    walletAddress: "0xf39Fd6e51aad88F6F4ce6aB8827279cffFb92266",
    nativeNetwork: "veil",
    routingFeeBps: 0,
  },
});
if (!order.accepted || !order.veilTxHash) throw new Error(`/orders: ${JSON.stringify(order)}`);
console.log(`4) native order settled ${order.veilTxHash}`);

const poly = await fetch(`${ROUTER}/orders`, {
  method: "POST",
  headers: { "content-type": "application/json", "x-relay-secret": SECRET },
  body: JSON.stringify({
    marketId: "poly-demo",
    side: "buy",
    outcome: "yes",
    amountUsd: 10,
    walletAddress: "0xf39Fd6e51aad88F6F4ce6aB8827279cffFb92266",
    nativeNetwork: "polygon",
    routingFeeBps: 3,
  }),
});
const polyJson = await poly.json();
if (poly.status !== 501 || polyJson.accepted !== false) {
  throw new Error(`polygon passthrough should 501, got ${poly.status} ${JSON.stringify(polyJson)}`);
}
console.log("5) polygon passthrough catalog-only (501)");

console.log("TOP-TO-BOTTOM PASS");
