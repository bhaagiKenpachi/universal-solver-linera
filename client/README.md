## Running the Client

The client requires a seed phrase to derive chain-specific keys for transaction signing. You can run the client with:

```bash
go run main.go -seed-phrase "your twelve word seed phrase here"
```

Additional optional flags:
- `-solver-url`: Universal Solver service URL (default: http://localhost:8080/)
- `-solana-url`: Solana RPC endpoint (default: http://localhost:8899)
- `-ethereum-url`: Ethereum RPC endpoint (default: http://localhost:8545)

Example:
```bash
go run main.go \
  -seed-phrase "your twelve word seed phrase here" \
  -solver-url "http://custom-solver:8080" \
  -solana-url "http://custom-solana:8899" \
  -ethereum-url "http://custom-ethereum:8545"
```

**Important**: Keep your seed phrase secure and never share it. The seed phrase is used to derive private keys for both Ethereum and Solana chains.

## API Endpoints

### POST /faucet
Request tokens from the faucet for testing purposes.

Parameters:
- `chain`: Chain to request tokens from (`solana` or `ethereum`)
- `address`: Recipient address

Example:
```bash
# Request Solana tokens
curl -X POST "http://localhost:3000/faucet?chain=solana&address=YOUR_SOLANA_ADDRESS"

# Request Ethereum tokens
curl -X POST "http://localhost:3000/faucet?chain=ethereum&address=YOUR_ETH_ADDRESS"
```

Response:
```json
{
    "status": "success",
    "chain": "solana",
    "data": {
        "signature": "5UYoBkwP4UUxLm6LuYUZfsi2PJww2GXwVNhXBKCRLGUqQYN7MBHXBtxEgzqxH2Nf7FnQYYP2GNP3sABr82dhUv1D",
        "amount": "2 SOL",
        "address": "YOUR_SOLANA_ADDRESS"
    }
}
```

### GET /get_pool_address
Get the pool address for a specific chain.

Parameters:
- `chain`: Chain name (e.g., `ethereum` or `solana`)

Example:
```bash
curl "http://localhost:3000/get_pool_address?chain=ethereum"
```

Response:
```json
{
    "status": "success",
    "chain": "ethereum",
    "data": {
        "address": "0x1234567890abcdef1234567890abcdef12345678"
    }
}
```

### GET /fetch_balance
Get the balance for an address on a specific chain.

Parameters:
- `chain`: Chain name (`solana` or `ethereum`)
- `address`: Address to check balance for

Example:
```bash
# Get Solana balance
curl "http://localhost:3000/fetch_balance?chain=solana&address=YOUR_SOLANA_ADDRESS"

# Get Ethereum balance
curl "http://localhost:3000/fetch_balance?chain=ethereum&address=YOUR_ETH_ADDRESS"
```

Response:
```json
{
    "status": "success",
    "chain": "ethereum",
    "data": {
        "address": "0x742d35Cc6634C0532925a3b844Bc454e4438f44e",
        "amount": 1.5,
        "symbol": "ETH"
    }
}
```

### GET /quote_swap
Get a quote for swapping tokens between chains.

Parameters:
- `fromChain`: Source chain (e.g., `ethereum` or `solana`)
- `toChain`: Destination chain
- `fromAmount`: Amount to swap in source chain's native units

Example:
```bash
curl "http://localhost:3000/quote_swap?fromChain=ethereum&toChain=solana&fromAmount=1.5"
```

Response:
```json
{
    "status": "success",
    "data": {
        "fromChain": "ethereum",
        "toChain": "solana",
        "fromAmount": 1.5,
        "toAmount": 210.75,
        "exchangeRate": 140.5
    }
}
``` 

## Deploy Bytecode

### POST /deploy_bytecode

This endpoint publishes the solver contract and service bytecode using the Linera CLI.

#### Request Body

The request body should be a JSON object with the following structure:

```json
{
  "contractWasm": "base64_encoded_contract_wasm_bytes",
  "serviceWasm": "base64_encoded_service_wasm_bytes"
}
```

- `contractWasm`: Base64 encoded bytes of the contract WASM file.
- `serviceWasm`: Base64 encoded bytes of the service WASM file.

#### Response

On success, the response will be a JSON object with the following structure:

```json
{
  "status": "success",
  "data": {
    "bytecodeId": "e476187f6ddfeb9d588c7b45d3df334d5501d6499b3f9ad5595cae86cce16a65"
  }
}
```

In case of an error, the response will contain an error message:

```json
{
  "status": "error",
  "message": "Error message here"
}
```

#### Example Request

```bash
# First, encode your WASM files to base64
CONTRACT_WASM=$(base64 examples/target/wasm32-unknown-unknown/release/solver_contract.wasm)
SERVICE_WASM=$(base64 examples/target/wasm32-unknown-unknown/release/solver_service.wasm)

# Then make the request
curl -X POST http://localhost:3001/deploy_bytecode \
  -H "Content-Type: application/json" \
  -d "{
    \"contractWasm\": \"$CONTRACT_WASM\",
    \"serviceWasm\": \"$SERVICE_WASM\"
  }"
```

#### Notes

- The WASM content must be base64 encoded.
- The endpoint requires the Linera CLI to be installed and available in the system PATH.
- The server must have permission to execute the Linera CLI.
- Temporary files are created and cleaned up automatically.