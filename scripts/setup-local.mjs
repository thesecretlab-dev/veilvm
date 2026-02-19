/**
 * Setup VeilVM local subnet + blockchain on a single-node AvalancheGo (v1.13.x)
 *
 * Flow:
 *   1. Get live nodeID + BLS POP from info.getNodeID
 *   2. Check primary validator set
 *   3. If missing → newAddPermissionlessValidatorTx, wait Committed + active
 *   4. Create subnet
 *   5. Add same nodeID as subnet validator
 *   6. Create chain
 *   7. Print restart instructions with --track-subnets
 *
 * Uses the pre-funded ewoq key on local networks.
 * Requires @avalabs/avalanchejs ^4.2.0
 */
import { readFileSync } from 'fs';
import { Context, pvm, utils, addTxSignatures, secp256k1 } from '@avalabs/avalanchejs';

const NODE_URL = process.env.NODE_URL || 'http://127.0.0.1:9660';
const VM_ID = 'u9GgvekeunSwK4TPF4jj7xLsW1LKkd1Uv9VQZo2SGfrwkejsK';
const PRIMARY_SUBNET = '11111111111111111111111111111111LpoYY';

// ewoq pre-funded key (standard for local Avalanche networks)
const EWOQ_PK_HEX = '56289e99c94b6912bfc12adc093c9b51124f0dc54ac7a766b2bc5ccf558d8027';
const EWOQ_P_ADDR = 'P-local18jma8ppw3nhx5r4ap8clazz0dps7rv5u00z96u';

// 2000 AVAX minimum for primary network validator
const MIN_VALIDATOR_STAKE = 2_000_000_000_000n;
// Delegation fee: 2% (20000 = 2.0000%)
const DELEGATION_FEE = 20000;

const sleep = ms => new Promise(r => setTimeout(r, ms));

/**
 * Sign ALL credentials on an unsigned tx (both UTXO inputs AND subnet auth).
 * addTxSignatures only covers UTXO inputs where the pubkey matches.
 * This helper signs the tx hash once, then injects the signature into every
 * credential slot that needs it.
 */
async function signAllCredentials(unsignedTx, privateKeyBytes) {
  // Sign UTXO input credentials
  await addTxSignatures({ unsignedTx, privateKeys: [privateKeyBytes] });

  // Sign subnet auth credential (last in the list)
  // secp256k1.sign returns Uint8Array(65) directly — do NOT wrap in Signature
  const sig = await secp256k1.sign(unsignedTx.toBytes(), privateKeyBytes);
  const authCredIndex = unsignedTx.getCredentials().length - 1;
  unsignedTx.addSignatureAt(sig, authCredIndex, 0);
}

// ─── RPC helpers ───────────────────────────────────────────────────────────

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

async function waitForHealth() {
  console.log('Waiting for node health...');
  for (let i = 0; i < 60; i++) {
    try {
      const res = await fetch(`${NODE_URL}/ext/health`);
      const data = await res.json();
      if (data.healthy) { console.log('Node healthy!'); return; }
    } catch {}
    await sleep(2000);
  }
  throw new Error('Node did not become healthy in 120s');
}

/** Get P-chain timestamp (not wall clock) */
async function getChainTime() {
  const { timestamp } = await rpc('/ext/bc/P', 'platform.getTimestamp');
  return BigInt(Math.floor(new Date(timestamp).getTime() / 1000));
}

/** Fetch feeState with local-network warmup guard */
async function getFeeState() {
  const result = await rpc('/ext/bc/P', 'platform.getFeeState');
  const capacity = BigInt(result.capacity);
  const excess = BigInt(result.excess);
  const price = BigInt(result.price);

  // On fresh local networks, capacity can be 0 before first block.
  // Apply a safe floor so tx builders don't divide-by-zero.
  if (capacity === 0n) {
    console.log('  ⚠ feeState.capacity=0 (no blocks yet), applying local floor');
    return { capacity: 1_000_000n, excess: 0n, price: price || 1n, timestamp: result.timestamp };
  }
  return { capacity, excess, price: price || 1n, timestamp: result.timestamp };
}

/** Fresh UTXOs for ewoq */
async function freshUTXOs(api) {
  const { utxos } = await api.getUTXOs({ addresses: [EWOQ_P_ADDR] });
  return utxos;
}

async function waitForTx(api, txID, label) {
  for (let i = 0; i < 60; i++) {
    await sleep(2000);
    try {
      const s = await api.getTxStatus({ txID });
      if (s.status === 'Committed') { console.log(`  ✓ ${label} committed`); return; }
      if (s.status === 'Dropped' || s.status === 'Aborted') {
        throw new Error(`${label} failed: ${s.reason || s.status}`);
      }
      console.log(`  … ${label}: ${s.status}`);
    } catch (e) {
      if (e.message?.includes('failed')) throw e;
    }
  }
  throw new Error(`${label} timed out after 120s`);
}

// ─── Main flow ─────────────────────────────────────────────────────────────

async function main() {
  await waitForHealth();

  // ── Step 1: Get our node identity ──
  console.log('\n═══ Step 1: Node Identity ═══');
  const { nodeID, nodePOP } = await rpc('/ext/info', 'info.getNodeID');
  console.log(`  NodeID:    ${nodeID}`);
  console.log(`  BLS PubKey: ${nodePOP.publicKey.slice(0, 20)}…`);
  console.log(`  BLS POP:    ${nodePOP.proofOfPossession.slice(0, 20)}…`);

  const context = await Context.getContextFromURI(NODE_URL);
  const api = new pvm.PVMApi(NODE_URL);
  const pkBytes = utils.hexToBuffer(EWOQ_PK_HEX);
  const addrBytes = utils.bech32ToBytes(EWOQ_P_ADDR);

  // ── Step 2: Check primary validator set ──
  console.log('\n═══ Step 2: Check Primary Validators ═══');
  const { validators } = await rpc('/ext/bc/P', 'platform.getCurrentValidators', {
    subnetID: PRIMARY_SUBNET,
  });
  const isAlreadyPrimary = validators.some(v => v.nodeID === nodeID);
  console.log(`  Primary validators: ${validators.length}`);
  console.log(`  Our node in set:    ${isAlreadyPrimary}`);

  let primaryStartTime;
  let primaryEndTime;

  if (isAlreadyPrimary) {
    const ours = validators.find(v => v.nodeID === nodeID);
    if (!ours) throw new Error('Node reported as primary but validator record was not found');
    primaryStartTime = BigInt(ours.startTime);
    primaryEndTime = BigInt(ours.endTime);
    console.log(`  Primary startTime:  ${primaryStartTime}`);
    console.log(`  Primary endTime:    ${primaryEndTime}`);
  } else {
    // ── Step 3: Add as primary validator ──
    console.log('\n═══ Step 3: Add Primary Validator ═══');

    const chainTime = await getChainTime();
    const startTime = chainTime + 30n;

    // Match genesis validator endTime (local networks require longer staking periods)
    const genesisEndTime = BigInt(validators[0].endTime);
    primaryStartTime = startTime;
    primaryEndTime = genesisEndTime;

    console.log(`  Chain time: ${chainTime}`);
    console.log(`  Start:      ${startTime}`);
    console.log(`  End:        ${primaryEndTime}`);
    console.log(`  Stake:      ${MIN_VALIDATOR_STAKE} nAVAX`);

    const utxos = await freshUTXOs(api);
    console.log(`  UTXOs:      ${utxos.length}`);

    const feeState = await getFeeState();

    const addValTx = pvm.newAddPermissionlessValidatorTx({
      utxos,
      fromAddressesBytes: [addrBytes],
      changeAddressesBytes: [addrBytes],
      nodeId: nodeID,
      subnetId: PRIMARY_SUBNET,
      start: startTime,
      end: primaryEndTime,
      weight: MIN_VALIDATOR_STAKE,
      publicKey: utils.hexToBuffer(nodePOP.publicKey),
      signature: utils.hexToBuffer(nodePOP.proofOfPossession),
      rewardAddresses: [addrBytes],
      delegatorRewardsOwner: [addrBytes],
      shares: DELEGATION_FEE,
      feeState,
    }, context);

    await addTxSignatures({ unsignedTx: addValTx, privateKeys: [pkBytes] });
    const { txID } = await api.issueSignedTx(addValTx.getSignedTx());
    console.log(`  TX: ${txID}`);
    await waitForTx(api, txID, 'AddPermissionlessValidator');

    // Wait for validator to become active
    console.log('  Waiting for validator to appear in current set...');
    for (let i = 0; i < 60; i++) {
      await sleep(3000);
      const { validators: current } = await rpc('/ext/bc/P', 'platform.getCurrentValidators', {
        subnetID: PRIMARY_SUBNET,
      });
      if (current.some(v => v.nodeID === nodeID)) {
        const ours = current.find(v => v.nodeID === nodeID);
        if (!ours) throw new Error('Primary validator appeared but record lookup failed');
        primaryStartTime = BigInt(ours.startTime);
        primaryEndTime = BigInt(ours.endTime);
        console.log('  ✓ Node is now a primary validator!');
        console.log(`  Primary startTime:  ${primaryStartTime}`);
        console.log(`  Primary endTime:    ${primaryEndTime}`);
        break;
      }
      if (i === 59) throw new Error('Node never appeared in primary validator set');
    }
  }

  // ── Step 4: Create Subnet ──
  console.log('\n═══ Step 4: Create Subnet ═══');
  {
    const utxos = await freshUTXOs(api);
    const feeState = await getFeeState();

    const createSubnetTx = pvm.newCreateSubnetTx({
      utxos,
      fromAddressesBytes: [addrBytes],
      changeAddressesBytes: [addrBytes],
      subnetOwners: [addrBytes],
      threshold: 1,
      feeState,
    }, context);

    await addTxSignatures({ unsignedTx: createSubnetTx, privateKeys: [pkBytes] });
    const { txID } = await api.issueSignedTx(createSubnetTx.getSignedTx());
    console.log(`  Subnet TX: ${txID}`);
    await waitForTx(api, txID, 'CreateSubnet');
    var subnetID = txID;
  }

  // ── Step 5: Add subnet validator (same nodeID) ──
  console.log('\n═══ Step 5: Add Subnet Validator ═══');
  {
    const chainTime = await getChainTime();
    // Subnet validator window must be strictly inside primary validator window.
    const desiredStart = chainTime + 30n;
    const earliestInsidePrimary = primaryStartTime + 1n;
    const startTime = desiredStart > earliestInsidePrimary ? desiredStart : earliestInsidePrimary;
    const endTime = primaryEndTime - 60n;

    if (startTime <= primaryStartTime) {
      throw new Error(`HARD FAIL: subnet startTime ${startTime} must be > primary startTime ${primaryStartTime}`);
    }
    if (startTime >= endTime) {
      throw new Error(`HARD FAIL: subnet window [${startTime}, ${endTime}] is invalid (outside primary window ending ${primaryEndTime})`);
    }
    if (endTime >= primaryEndTime) {
      throw new Error(`HARD FAIL: subnet endTime ${endTime} must be < primary endTime ${primaryEndTime}`);
    }
    if (endTime > primaryEndTime) {
      throw new Error(`HARD FAIL: subnet endTime ${endTime} exceeds primary endTime ${primaryEndTime}`);
    }

    console.log(`  NodeID:  ${nodeID}`);
    console.log(`  Primary: [${primaryStartTime}, ${primaryEndTime}]`);
    console.log(`  Start:   ${startTime}`);
    console.log(`  End:     ${endTime}`);

    const utxos = await freshUTXOs(api);
    const feeState = await getFeeState();

    const addSubnetValTx = pvm.newAddSubnetValidatorTx({
      utxos,
      fromAddressesBytes: [addrBytes],
      changeAddressesBytes: [addrBytes],
      nodeId: nodeID,
      start: startTime,
      end: endTime,
      weight: 100n,
      subnetId: subnetID,
      subnetAuth: [0],
      feeState,
    }, context);

    await signAllCredentials(addSubnetValTx, pkBytes);
    const { txID } = await api.issueSignedTx(addSubnetValTx.getSignedTx());
    console.log(`  TX: ${txID}`);
    await waitForTx(api, txID, 'AddSubnetValidator');

    // Wait for subnet validator to become active
    console.log('  Waiting for subnet validator to activate...');
    for (let i = 0; i < 60; i++) {
      await sleep(3000);
      const { validators: subnetVals } = await rpc('/ext/bc/P', 'platform.getCurrentValidators', {
        subnetID,
      });
      if (subnetVals.some(v => v.nodeID === nodeID)) {
        console.log('  ✓ Node is now a subnet validator!');
        break;
      }
      if (i === 59) throw new Error('Node never appeared in subnet validator set');
    }
  }

  // ── Step 6: Create Blockchain ──
  console.log('\n═══ Step 6: Create Blockchain ═══');
  let chainID;
  {
    const utxos = await freshUTXOs(api);
    const feeState = await getFeeState();

    const genesisData = JSON.parse(
      readFileSync(new URL('../genesis.json', import.meta.url), 'utf-8')
    );

    const createChainTx = pvm.newCreateChainTx({
      utxos,
      fromAddressesBytes: [addrBytes],
      changeAddressesBytes: [addrBytes],
      subnetId: subnetID,
      chainName: 'VEIL',
      vmId: VM_ID,
      fxIds: [],
      genesisData,
      subnetAuth: [0],
      feeState,
    }, context);

    await signAllCredentials(createChainTx, pkBytes);
    const { txID } = await api.issueSignedTx(createChainTx.getSignedTx());
    console.log(`  Chain TX: ${txID}`);
    await waitForTx(api, txID, 'CreateBlockchain');
    chainID = txID;
  }

  // ── Step 7: Output results + restart instructions ──
  console.log('\n╔══════════════════════════════════════╗');
  console.log('║       VEIL CHAIN DEPLOYED            ║');
  console.log('╠══════════════════════════════════════╣');
  console.log(`║ Node ID:   ${nodeID}`);
  console.log(`║ Subnet ID: ${subnetID}`);
  console.log(`║ Chain ID:  ${chainID}`);
  console.log(`║ RPC URL:   ${NODE_URL}/ext/bc/${chainID}/rpc`);
  console.log(`║ API URL:   ${NODE_URL}/ext/bc/${chainID}/veilapi`);
  console.log('╚══════════════════════════════════════╝');

  console.log('\n⚠ NEXT: Restart node with --track-subnets to activate the VM:');
  console.log(`\n  Add to docker-compose command:`);
  console.log(`    - --track-subnets=${subnetID}`);
  console.log(`\n  Then: docker compose -f docker-compose.local.yml up -d`);
  console.log(`\n  Verify chain RPC:`);
  console.log(`    curl -s -X POST ${NODE_URL}/ext/bc/${chainID}/rpc \\`);
  console.log(`      -H 'content-type: application/json' \\`);
  console.log(`      -d '{"jsonrpc":"2.0","id":1,"method":"veilapi.ping","params":{}}'`);
}

main().catch(e => { console.error('✗', e.message || e); process.exit(1); });
