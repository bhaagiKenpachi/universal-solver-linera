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