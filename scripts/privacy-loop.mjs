#!/usr/bin/env node
/**
 * Local privacy loop: create → opaque commit → reveal → groth16 proof → clear.
 * Requires :9660 + :9098. Not Fuji. Does not claim mempool gossip privacy.
 */
const ROUTER = process.env.ORDER_ROUTER_URL || "http://127.0.0.1:9098";
const SECRET = process.env.ORDER_ROUTER_RELAY_SECRET || "local-dev-secret";

async function call(path, { method = "GET", body, timeoutMs = 120_000 } = {}) {
  const headers = { "x-relay-secret": SECRET };
  if (body) headers["content-type"] = "application/json";
  const ac = new AbortController();
  const t = setTimeout(() => ac.abort(), timeoutMs);
  try {
    const res = await fetch(`${ROUTER}${path}`, {
      method,
      headers,
      body: body ? JSON.stringify(body) : undefined,
      signal: ac.signal,
    });
    const json = await res.json().catch(() => ({}));
    if (!res.ok) {
      throw new Error(`${method} ${path} ${res.status}: ${JSON.stringify(json)}`);
    }
    return json;
  } finally {
    clearTimeout(t);
  }
}

async function waitProver(ms) {
  const start = Date.now();
  while (Date.now() - start < ms) {
    const h = await call("/health");
    if (h.proverReady) return h;
    await new Promise((r) => setTimeout(r, 2000));
  }
  throw new Error("groth16 prover not ready");
}

const health = await waitProver(180_000);
console.log(`1) router ok chain=${health.chainId} proverReady=${health.proverReady}`);

const created = await call("/native/create-market", {
  method: "POST",
  body: { question: "privacy-loop native", outcomes: 2, creatorBond: 1 },
});
if (!created.accepted || !created.marketId) throw new Error(`create-market: ${JSON.stringify(created)}`);
console.log(`2) market ${created.marketId}`);

const wallet = "0xf39fd6e51aad88f6f4ce6ab8827279cfffb92266";
const nonce = "0x" + "cd".repeat(32);
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
const ANVIL_PK =
  process.env.EVM_RELAY_EXECUTOR_PRIVATE_KEY ||
  "0xac0974bec39a17e36ba4a6b4d238ff944bacb478cbed5efcae784d7bf4f2ff80";
const { spawnSync } = await import("node:child_process");
const signed = spawnSync(CAST, ["wallet", "sign", "--private-key", ANVIL_PK, message], {
  encoding: "utf8",
});
if (signed.status !== 0) {
  throw new Error(`cast wallet sign: ${signed.stderr || signed.stdout}`);
}
const order = await call("/orders", {
  method: "POST",
  body: {
    marketId: created.marketId,
    side: "buy",
    outcome: "yes",
    amountUsd,
    walletAddress: wallet,
    walletNonce: nonce,
    walletSignature: (signed.stdout || "").trim(),
    nativeNetwork: "veil",
    routingFeeBps: 0,
  },
});
if (!order.accepted || order.status !== "committed" || !order.veilTxHash) {
  throw new Error(`/orders: ${JSON.stringify(order)}`);
}
if (typeof order.windowId !== "number" || order.windowId <= 0) {
  throw new Error(`missing windowId: ${JSON.stringify(order)}`);
}
console.log(`3) committed ${order.veilTxHash} window=${order.windowId} (not cleared)`);

const settled = await call("/native/settle-batch", {
  method: "POST",
  timeoutMs: 180_000,
  body: { marketId: created.marketId, windowId: order.windowId },
});
if (!settled.accepted || settled.status !== "cleared") {
  throw new Error(`settle: ${JSON.stringify(settled)}`);
}
if (!settled.revealTxHash || !settled.proofTxHash || !settled.clearTxHash) {
  throw new Error(`missing settle txs: ${JSON.stringify(settled)}`);
}
console.log(`4) revealed ${settled.revealTxHash}`);
console.log(`5) proof ${settled.proofTxHash} proveMs=${settled.proveMs}`);
console.log(`6) cleared ${settled.clearTxHash} fills=${settled.fillsHash}`);
console.log("PRIVACY-LOOP PASS (local commit/reveal/proof/clear). Tx gossip is VTG2 2-of-3 (single key cannot decrypt; RPC ingest still builds blocks). Circuit still digest-binds public slots.");
