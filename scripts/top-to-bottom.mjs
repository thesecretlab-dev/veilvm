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

const wallet = "0xf39fd6e51aad88f6f4ce6ab8827279cfffb92266";
const nonce = "0x" + "ab".repeat(32);
const amountUsd = 25;
const message = [
  "VEIL native order v1",
  `chain:${health.chainId}`,
  `market:${created.marketId}`,
  "side:buy",
  "outcome:yes",
  `amountUsd:${amountUsd.toFixed(8)}`,
  `wallet:${wallet}`,
  `nonce:${nonce}`,
].join("\n");
const CAST = process.env.CAST_BIN || "cast";
const ANVIL_PK = process.env.EVM_RELAY_EXECUTOR_PRIVATE_KEY ||
  "0xac0974bec39a17e36ba4a6b4d238ff944bacb478cbed5efcae784d7bf4f2ff80";
const { spawnSync } = await import("node:child_process");
const signed = spawnSync(CAST, ["wallet", "sign", "--private-key", ANVIL_PK, message], {
  encoding: "utf8",
});
if (signed.status !== 0) {
  throw new Error(`cast wallet sign: ${signed.stderr || signed.stdout}`);
}
const walletSignature = (signed.stdout || "").trim();

const order = await call("/orders", {
  method: "POST",
  body: {
    marketId: created.marketId,
    side: "buy",
    outcome: "yes",
    amountUsd,
    walletAddress: wallet,
    walletNonce: nonce,
    walletSignature,
    nativeNetwork: "veil",
    routingFeeBps: 0,
  },
});
if (!order.accepted || !order.veilTxHash) throw new Error(`/orders: ${JSON.stringify(order)}`);
if (order.status && order.status !== "committed") {
  throw new Error(`/orders expected committed, got ${JSON.stringify(order)}`);
}
console.log(`4) native order committed ${order.veilTxHash} window=${order.windowId ?? "?"}`);

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
