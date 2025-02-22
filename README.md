# Universal Solver

Architecture:
  1. Add pools
    - add l1 token with address containing balance
  2. Add pool Addresses list
    - add address along with balance
  3. Transaction database
    - TxInItem, TxOutItem
      - tx_hash
      - from
      - to
      - value
      - gas
      - gas_price
      - chain
      - token
    - status 
      - pending
      - processed
      - failed
      - success
  

## GraphQL API Reference

### Queries

#### Get File by ID
```graphql
query {
  getFileSolverApp(id: "file_id_here") {
    solverFileId
    owner
    name
    payload
  }
}
```

#### Get Files by Owner
```graphql
query {
  getFilesByOwner(owner: "User:21cf9e655850b761c4577f6d1324b2956c7a3d39f3733ca0e05c4aad381da4a5") {
    solverFileId
    owner
    name
    payload
  }
}
```

#### Get Pool Address
```graphql
query {
  getPoolAddress(chainName: "ethereum") {
    address
  }
}
```

#### Get All Pool Addresses
```graphql
query {
  getAllPoolAddresses {
    chainName
    address
  }
}
```

#### JSON Utilities
```graphql
# Convert JSON to bytes
query {
  jsonToBytes(jsonStr: "{\"key\": \"value\"}")
}

# Convert bytes to JSON
query {
  bytesToJson(bytes: [123, 34, 107, 101, 121, 34, 58, 32, 34, 118, 97, 108, 117, 101, 34, 125])
}
```

#### Calculate Swap
```graphql
query {
    calculateSwap(
        fromToken: "ETH",
        toToken: "SOL",
        amount: 1000000000
    ) {
        fromToken
        toToken
        fromAmount
        toAmount
        exchangeRate
    }
}
```

### Mutations

#### Add File
```graphql
mutation {
  addFile(
    owner: "User:21cf9e655850b761c4577f6d1324b2956c7a3d39f3733ca0e05c4aad381da4a5"
    name: "example.json"
    blobHash: "hash_here"
  )
}
```

#### Add Pool Address
```graphql
mutation {
  addPoolAddress(
    chainName: "ethereum"
    address: "0x123..."
  )
}
```

#### Remove Pool Address
```graphql
mutation {
  removePoolAddress(
    chainName: "ethereum"
  )
}
```

#### Execute Swap
```graphql
mutation {
    executeSwap(
        fromToken: "ETH",
        toToken: "SOL",
        amount: 1000000000,
        destinationAddress: "0x123..."
    )
}
```

## HTTP API Reference

The service also provides a REST API for transaction-related operations.

### Endpoints

#### POST /post_tx_hash
Get transaction details by hash and optionally execute a swap.

Request:
```bash
# Get transaction details only
curl -X POST "http://localhost:3000/post_tx_hash?chain=ethereum&txHash=0x123..."

# Get transaction details and execute swap
curl -X POST "http://localhost:3000/post_tx_hash?chain=ethereum&txHash=0x123...&toToken=SOL&destinationAddress=0xabc..."
```

Response (with swap):
```json
{
  "status": "success",
  "chain": "ethereum",
  "data": {
    "hash": "0x123...",
    "blockHash": "0x456...",
    "blockNumber": "12345",
    "from": "0x789...",
    "to": "0xabc...",
    "value": "1000000000000000000",
    "gasPrice": "20000000000",
    "gas": "21000",
    "nonce": "5",
    "input": "0x",
    "transactionIndex": "0",
    "v": "0x1b",
    "r": "0xdef...",
    "s": "0x123..."
  },
  "swap_result": {
    "tx_hash": "0x789...",
    "swap_result": {
      "from_token": "ETH",
      "to_token": "SOL",
      "from_amount": 1000000000000000000,
      "to_amount": 25000000000,
      "exchange_rate": 25.0
    },
    "status": "pending"
  }
}
```

## Development

### Building
```

## Running the Client

The client can be configured using command-line flags or environment variables:

### Command-line Flags
```bash
go run main.go [flags]

Flags:
  -solver-url string    Universal Solver service URL (default "http://localhost:8080/")
  -solana-url string    Solana RPC endpoint (default "http://localhost:8899")
  -ethereum-url string  Ethereum RPC endpoint (default "http://localhost:8545")
```

### Environment Variables
- `SOLVER_URL`: Universal Solver service URL
- `SOLANA_RPC`: Solana RPC endpoint
- `ETHEREUM_RPC`: Ethereum RPC endpoint
- `PORT`: Server port (default "3000")

### Examples

1. Using command-line flags:
```bash
go run main.go \
  -solver-url="http://solver.example.com/" \
  -solana-url="http://solana.example.com" \
  -ethereum-url="http://ethereum.example.com"
```

2. Using environment variables:
```bash
export SOLVER_URL="http://solver.example.com/"
export SOLANA_RPC="http://solana.example.com"
export ETHEREUM_RPC="http://ethereum.example.com"
export PORT="8000"
go run main.go
```