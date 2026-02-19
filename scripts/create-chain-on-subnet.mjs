/**
 * Create a fresh VEIL chain on an already-tracked subnet.
 *
 * Use this when the current VEIL chain fee payer is depleted and you want
 * a fresh genesis-funded chain without wiping node volumes.
 *
 * Env overrides:
 * - NODE_URL   (default: http://127.0.0.1:9660)
 * - SUBNET_ID  (default: local tracked subnet)
 */
import { readFileSync } from 'fs';
import { Context, pvm, utils, addTxSignatures, secp256k1 } from '@avalabs/avalanchejs';

const NODE_URL = process.env.NODE_URL || 'http://127.0.0.1:9660';
const EXISTING_SUBNET_ID = process.env.SUBNET_ID || '2AWc8cibQ5tZgTiMo9KoRaZHc3TV3pEcBRsDBr2DiMZbPNSFm9';
const VM_ID = 'u9GgvekeunSwK4TPF4jj7xLsW1LKkd1Uv9VQZo2SGfrwkejsK';
const EWOQ_PK_HEX = '56289e99c94b6912bfc12adc093c9b51124f0dc54ac7a766b2bc5ccf558d8027';
const EWOQ_P_ADDR = 'P-local18jma8ppw3nhx5r4ap8clazz0dps7rv5u00z96u';

const sleep = ms => new Promise(r => setTimeout(r, ms));

async function rpc(endpoint, method, params = {}) {
  const res = await fetch(`${NODE_URL}${endpoint}`, {
    method: 'POST',
    headers: { 'content-type': 'application/json' },
    body: JSON.stringify({ jsonrpc: '2.0', id: 1, method, params }),
  });
  const json = await res.json();
  if (json.error) throw new Error(`${method}: ${json.error.message}`);
  return json.result;
}

async function getFeeState() {
  const result = await rpc('/ext/bc/P', 'platform.getFeeState');
  const capacity = BigInt(result.capacity);
  const price = BigInt(result.price);
  if (capacity === 0n) {
    return { capacity: 1_000_000n, excess: 0n, price: price || 1n, timestamp: result.timestamp };
  }
  return { capacity, excess: BigInt(result.excess), price: price || 1n, timestamp: result.timestamp };
}

async function waitForTx(api, txID, label) {
  for (let i = 0; i < 90; i++) {
    await sleep(2000);
    const s = await api.getTxStatus({ txID });
    if (s.status === 'Committed') return;
    if (s.status === 'Dropped' || s.status === 'Aborted') {
      throw new Error(`${label} failed: ${s.reason || s.status}`);
    }
  }
  throw new Error(`${label} timed out`);
}

async function signAllCredentials(unsignedTx, privateKeyBytes) {
  await addTxSignatures({ unsignedTx, privateKeys: [privateKeyBytes] });
  const sig = await secp256k1.sign(unsignedTx.toBytes(), privateKeyBytes);
  const authCredIndex = unsignedTx.getCredentials().length - 1;
  unsignedTx.addSignatureAt(sig, authCredIndex, 0);
}

async function main() {
  const context = await Context.getContextFromURI(NODE_URL);
  const api = new pvm.PVMApi(NODE_URL);
  const pkBytes = utils.hexToBuffer(EWOQ_PK_HEX);
  const addrBytes = utils.bech32ToBytes(EWOQ_P_ADDR);

  const { utxos } = await api.getUTXOs({ addresses: [EWOQ_P_ADDR] });
  const feeState = await getFeeState();
  const genesisData = JSON.parse(readFileSync(new URL('../genesis.json', import.meta.url), 'utf-8'));

  const createChainTx = pvm.newCreateChainTx({
    utxos,
    fromAddressesBytes: [addrBytes],
    changeAddressesBytes: [addrBytes],
    subnetId: EXISTING_SUBNET_ID,
    chainName: `VEIL${Date.now()}`,
    vmId: VM_ID,
    fxIds: [],
    genesisData,
    subnetAuth: [0],
    feeState,
  }, context);

  await signAllCredentials(createChainTx, pkBytes);
  const { txID } = await api.issueSignedTx(createChainTx.getSignedTx());
  console.log(`CreateBlockchain TX: ${txID}`);
  await waitForTx(api, txID, 'CreateBlockchain');

  console.log(`NEW_CHAIN_ID=${txID}`);
  console.log(`NEW_CHAIN_COREAPI=${NODE_URL}/ext/bc/${txID}/coreapi`);
  console.log(`NEW_CHAIN_VEILAPI=${NODE_URL}/ext/bc/${txID}/veilapi`);
}

main().catch((e) => {
  console.error(e.message || e);
  process.exit(1);
});
