#!/bin/bash
# Wait for node to be ready, then create Subnet + Blockchain for VeilVM
set -euo pipefail

NODE_URL="${NODE_URL:-http://localhost:9650}"
VM_ID="u9GgvekeunSwK4TPF4jj7xLsW1LKkd1Uv9VQZo2SGfrwkejsK"

echo "Waiting for node to bootstrap..."
for i in $(seq 1 60); do
  if curl -sf "$NODE_URL/ext/health" | grep -q '"healthy":true'; then
    echo "Node is healthy!"
    break
  fi
  echo "  Attempt $i/60..."
  sleep 2
done

# Get the node ID
NODE_ID=$(curl -sf -X POST "$NODE_URL/ext/info" \
  -H 'content-type: application/json' \
  -d '{"jsonrpc":"2.0","id":1,"method":"info.getNodeID"}' | \
  python3 -c "import sys,json; print(json.load(sys.stdin)['result']['nodeID'])")
echo "Node ID: $NODE_ID"

# Import the pre-funded key on the local network (password: veiltest)
# On a local network, the P-chain has a pre-funded key
echo "Creating user..."
curl -sf -X POST "$NODE_URL/ext/keystore" \
  -H 'content-type: application/json' \
  -d '{"jsonrpc":"2.0","id":1,"method":"keystore.createUser","params":{"username":"veiladmin","password":"veiltest123!"}}'

# Import the default pre-funded private key for local networks
echo ""
echo "Importing pre-funded key to P-chain..."
curl -sf -X POST "$NODE_URL/ext/bc/P" \
  -H 'content-type: application/json' \
  -d '{"jsonrpc":"2.0","id":1,"method":"platform.importKey","params":{"username":"veiladmin","password":"veiltest123!","privateKey":"PrivateKey-ewoqjP7PxY4yr3iLTpLisriqt94hdyDFNgchSxGGztUrTXtNN"}}'

echo ""
echo "Creating subnet..."
SUBNET_RESULT=$(curl -sf -X POST "$NODE_URL/ext/bc/P" \
  -H 'content-type: application/json' \
  -d '{
    "jsonrpc":"2.0","id":1,"method":"platform.createSubnet",
    "params":{
      "controlKeys":["P-local18jma8ppw3nhx5r4ap8clazz0dps7rv5u00z96u"],
      "threshold":1,
      "from":["P-local18jma8ppw3nhx5r4ap8clazz0dps7rv5u00z96u"],
      "changeAddr":"P-local18jma8ppw3nhx5r4ap8clazz0dps7rv5u00z96u",
      "username":"veiladmin","password":"veiltest123!"
    }
  }')
echo "Subnet result: $SUBNET_RESULT"
SUBNET_TX=$(echo "$SUBNET_RESULT" | python3 -c "import sys,json; print(json.load(sys.stdin)['result']['txID'])")
echo "Subnet TX (also SubnetID): $SUBNET_TX"

echo "Waiting for subnet tx to be accepted..."
sleep 5

# Add validator to the subnet
echo "Adding validator to subnet..."
curl -sf -X POST "$NODE_URL/ext/bc/P" \
  -H 'content-type: application/json' \
  -d "{
    \"jsonrpc\":\"2.0\",\"id\":1,\"method\":\"platform.addSubnetValidator\",
    \"params\":{
      \"nodeID\":\"$NODE_ID\",
      \"subnetID\":\"$SUBNET_TX\",
      \"startTime\":$(date -d '+1 minute' +%s),
      \"endTime\":$(date -d '+365 days' +%s),
      \"weight\":100,
      \"username\":\"veiladmin\",\"password\":\"veiltest123!\"
    }
  }"

echo ""
echo "Waiting for validator to be added..."
sleep 10

# Read genesis file
GENESIS_B64=$(base64 -w0 /root/genesis.json)

echo "Creating blockchain with VeilVM..."
CHAIN_RESULT=$(curl -sf -X POST "$NODE_URL/ext/bc/P" \
  -H 'content-type: application/json' \
  -d "{
    \"jsonrpc\":\"2.0\",\"id\":1,\"method\":\"platform.createBlockchain\",
    \"params\":{
      \"subnetID\":\"$SUBNET_TX\",
      \"vmID\":\"$VM_ID\",
      \"name\":\"VEIL\",
      \"genesisData\":\"$GENESIS_B64\",
      \"encoding\":\"base64\",
      \"from\":[\"P-local18jma8ppw3nhx5r4ap8clazz0dps7rv5u00z96u\"],
      \"changeAddr\":\"P-local18jma8ppw3nhx5r4ap8clazz0dps7rv5u00z96u\",
      \"username\":\"veiladmin\",\"password\":\"veiltest123!\"
    }
  }")
echo "Chain result: $CHAIN_RESULT"
CHAIN_ID=$(echo "$CHAIN_RESULT" | python3 -c "import sys,json; print(json.load(sys.stdin)['result']['txID'])")

echo ""
echo "=== VEIL CHAIN DEPLOYED ==="
echo "Subnet ID: $SUBNET_TX"
echo "Chain ID:  $CHAIN_ID"
echo "RPC URL:   $NODE_URL/ext/bc/$CHAIN_ID/rpc"
echo "API URL:   $NODE_URL/ext/bc/$CHAIN_ID/veilapi"
echo ""
