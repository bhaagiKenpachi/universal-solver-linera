package solver

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"math/big"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/common/hexutil"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/ethereum/go-ethereum/ethclient"
	"github.com/gagliardetto/solana-go"
	"github.com/gagliardetto/solana-go/programs/system"
	"github.com/gagliardetto/solana-go/rpc"
	"github.com/linera-protocol/examples/universal-solver/client/solver/keys"
	"github.com/mr-tron/base58"
)

// Add at the top with other package-level variables
var (
	// RPC endpoints
	EthereumRPC string
	SolanaRPC   string
	// Chain keys
	chainKeys *keys.ChainKeys
)

// Add a function to initialize RPC URLs
func InitRPCEndpoints(ethereumURL, solanaURL string) {
	EthereumRPC = ethereumURL
	SolanaRPC = solanaURL
	Logger.Printf("Initialized RPC endpoints - Ethereum: %s, Solana: %s", ethereumURL, solanaURL)
}

// InitKeys initializes the private keys from a seed phrase
func InitKeys(seedPhrase string) error {
	var err error
	chainKeys, err = keys.DeriveKeysFromSeedPhrase(seedPhrase)
	if err != nil {
		Logger.Printf("Failed to derive keys: %v", err)
		return fmt.Errorf("failed to derive keys: %w", err)
	}
	Logger.Printf("Successfully initialized chain keys")
	return nil
}

type Client struct {
	baseURL string
	http    *http.Client
}

func NewClient(baseURL string) *Client {
	Logger.Printf("Creating new solver client with base URL: %s", baseURL)
	return &Client{
		baseURL: baseURL,
		http:    &http.Client{},
	}
}

// GetSolanaTransaction fetches transaction details from Solana
func (c *Client) GetSolanaTransaction(_, txHash string) (interface{}, error) {
	// Prepare the JSON-RPC request
	requestBody := map[string]interface{}{
		"jsonrpc": "2.0",
		"id":      1,
		"method":  "getTransaction",
		"params": []interface{}{
			txHash,
			map[string]interface{}{
				"encoding":                       "json",
				"maxSupportedTransactionVersion": 0,
			},
		},
	}

	// Make the request with retries
	var response interface{}
	var err error
	for i := 0; i < 10; i++ {
		response, err = c.makeRPCRequest(SolanaRPC, requestBody)
		if responseMap, ok := response.(map[string]interface{}); ok {
			if responseMap["result"] == nil {
				time.Sleep(5 * time.Second)
				continue // Retry if result is nil
			}
		}

		if err == nil {
			break
		}
	}
	if err != nil {
		return nil, fmt.Errorf("failed to get Solana transaction after 10 retries: %w", err)
	}

	return response, nil
}

// GetEthereumTransaction fetches transaction details from Ethereum
func (c *Client) GetEthereumTransaction(_, txHash string) (interface{}, error) {
	client, err := ethclient.Dial(EthereumRPC)
	if err != nil {
		return nil, fmt.Errorf("failed to connect to Ethereum node: %w", err)
	}
	defer client.Close()

	hash := common.HexToHash(txHash)
	tx, isPending, err := client.TransactionByHash(context.Background(), hash)
	if err != nil {
		return nil, fmt.Errorf("failed to get Ethereum transaction: %w", err)
	}

	// Convert transaction to map for consistent response format
	return map[string]interface{}{
		"hash":      tx.Hash().Hex(),
		"value":     tx.Value().String(),
		"gas":       tx.Gas(),
		"gasPrice":  tx.GasPrice().String(),
		"nonce":     tx.Nonce(),
		"isPending": isPending,
	}, nil
}

func (c *Client) makeRPCRequest(endpoint string, requestBody interface{}) (interface{}, error) {
	jsonBody, err := json.Marshal(requestBody)
	if err != nil {
		return nil, err
	}

	req, err := http.NewRequest("POST", endpoint, bytes.NewBuffer(jsonBody))
	if err != nil {
		return nil, err
	}

	req.Header.Set("Content-Type", "application/json")

	resp, err := c.http.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	var result map[string]interface{}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, err
	}

	return result, nil
}

func (c *Client) GetFile(id string) (*SolverFile, error) {
	query := fmt.Sprintf(`{
		"query": "query { getFileSolverApp(id: \"%s\") { solverFileId owner name payload } }"
	}`, id)

	req, err := http.NewRequest("POST", c.baseURL, bytes.NewBuffer([]byte(query)))
	if err != nil {
		return nil, fmt.Errorf("error creating request: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")

	resp, err := c.http.Do(req)
	if err != nil {
		return nil, fmt.Errorf("error sending request: %w", err)
	}
	defer resp.Body.Close()

	var result GraphQLResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("error parsing response: %w", err)
	}

	return &result.Data.GetFileSolverApp, nil
}

func (c *Client) GetTransactionByHash(hash string) (*Transaction, error) {
	query := fmt.Sprintf(`{
		"query": "query { getTransaction(hash: \"%s\") { 
			hash
			blockHash
			blockNumber
			from
			to
			value
			gasPrice
			gas
			nonce
			input
			transactionIndex
			v
			r
			s
	 }}"
	}`, hash)

	req, err := http.NewRequest("POST", c.baseURL, bytes.NewBuffer([]byte(query)))
	if err != nil {
		return nil, fmt.Errorf("error creating request: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")

	resp, err := c.http.Do(req)
	if err != nil {
		return nil, fmt.Errorf("error sending request: %w", err)
	}
	defer resp.Body.Close()

	var result struct {
		Data struct {
			GetTransaction *Transaction `json:"getTransaction"`
		} `json:"data"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("error parsing response: %w", err)
	}

	return result.Data.GetTransaction, nil
}

// CalculateSwap calculates swap details without executing the swap
func (c *Client) CalculateSwap(fromToken, toToken string, amount float64) (*SwapResult, error) {
	// Prepare GraphQL query
	query := fmt.Sprintf(`{
		"query": "query { calculateSwap(fromToken:\"%s\",toToken:\"%s\",amount:%f) { fromToken toToken fromAmount toAmount exchangeRate } }"
	}`, fromToken, toToken, amount)

	// Create request
	req, err := http.NewRequest("POST", c.baseURL, bytes.NewBuffer([]byte(query)))
	if err != nil {
		return nil, fmt.Errorf("error creating request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")

	// Execute request
	resp, err := c.http.Do(req)
	if err != nil {
		return nil, fmt.Errorf("error executing request: %w", err)
	}
	defer resp.Body.Close()

	// Parse response
	var result struct {
		Data struct {
			CalculateSwap struct {
				FromToken    string  `json:"fromToken"`
				ToToken      string  `json:"toToken"`
				FromAmount   float64 `json:"fromAmount"`
				ToAmount     float64 `json:"toAmount"`
				ExchangeRate float64 `json:"exchangeRate"`
			} `json:"calculateSwap"`
		} `json:"data"`
		Errors []struct {
			Message string `json:"message"`
		} `json:"errors,omitempty"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("error decoding response: %w", err)
	}

	if len(result.Errors) > 0 {
		return nil, fmt.Errorf("GraphQL error: %s", result.Errors[0].Message)
	}

	return &SwapResult{
		FromToken:    result.Data.CalculateSwap.FromToken,
		ToToken:      result.Data.CalculateSwap.ToToken,
		FromAmount:   result.Data.CalculateSwap.FromAmount,
		ToAmount:     result.Data.CalculateSwap.ToAmount,
		ExchangeRate: result.Data.CalculateSwap.ExchangeRate,
	}, nil
}

// ExecuteSwap performs the swap operation
func (c *Client) ExecuteSwap(fromToken, toToken string, amount float64, destinationAddress string) (*SwapResponse, error) {
	// First calculate the swap
	swapResult, err := c.CalculateSwap(fromToken, toToken, amount)
	if err != nil {
		return nil, fmt.Errorf("failed to calculate swap: %w", err)
	}

	// Execute the swap mutation
	mutation := fmt.Sprintf(`{"query":"mutation calSwap{swap(fromToken:\"%s\",toToken:\"%s\",amount:\"%v\",destinationAddress:\"%s\")}"}`, fromToken, toToken, amount, destinationAddress)

	req, err := http.NewRequest("POST", c.baseURL, bytes.NewBuffer([]byte(mutation)))
	if err != nil {
		return nil, fmt.Errorf("error creating request: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")

	resp, err := c.http.Do(req)
	if err != nil {
		return nil, fmt.Errorf("error sending request: %w", err)
	}
	defer resp.Body.Close()

	var rawResponse map[string]interface{}
	if err := json.NewDecoder(resp.Body).Decode(&rawResponse); err != nil {
		return nil, fmt.Errorf("error parsing raw response: %w", err)
	}

	// Create properly structured result
	var result struct {
		Data   string `json:"data"`
		Errors []struct {
			Message string `json:"message"`
		} `json:"errors,omitempty"`
	}

	// Re-encode and decode to ensure proper type conversion
	jsonData, err := json.Marshal(rawResponse)
	if err != nil {
		return nil, fmt.Errorf("error re-encoding response: %w", err)
	}

	if err := json.Unmarshal(jsonData, &result); err != nil {
		return nil, fmt.Errorf("error parsing structured response: %w", err)
	}

	if len(result.Errors) > 0 {
		return nil, fmt.Errorf("GraphQL error: %s", result.Errors[0].Message)
	}

	swapResponse := &SwapResponse{
		TxHash:             result.Data,
		SwapResult:         *swapResult,
		Status:             "pending",
		DestinationAddress: destinationAddress,
	}

	// Prepare transaction for signing based on chain
	chain := c.determineChain(toToken)
	if err := c.PrepareTransaction(chain, swapResponse); err != nil {
		return nil, fmt.Errorf("failed to prepare transaction: %w", err)
	}

	// Sign the prepared transaction
	if err := c.SignTransaction(swapResponse); err != nil {
		return nil, fmt.Errorf("failed to sign transaction: %w", err)
	}

	// Submit the signed transaction
	if err := c.SubmitTransaction(swapResponse); err != nil {
		return nil, fmt.Errorf("failed to submit transaction: %w", err)
	}

	return swapResponse, nil
}

func (c *Client) determineChain(token string) string {
	switch token {
	case "ETH":
		return "ethereum"
	case "SOL":
		return "solana"
	default:
		return "unknown"
	}
}

// PrepareTransaction prepares a transaction for signing based on chain type
func (c *Client) PrepareTransaction(chain string, swap *SwapResponse) error {
	switch chain {
	case "ethereum":
		return c.prepareEthereumTransaction(swap)
	case "solana":
		return c.prepareSolanaTransaction(swap)
	default:
		return fmt.Errorf("unsupported chain: %s", chain)
	}
}

// GetAllPools fetches all pool addresses
func (c *Client) GetAllPools() ([]Pool, error) {
	query := `{"query":"query pools{getAllPools{chainName poolAddress}}"}`

	req, err := http.NewRequest("POST", c.baseURL, bytes.NewBuffer([]byte(query)))
	if err != nil {
		return nil, fmt.Errorf("error creating request: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")

	resp, err := c.http.Do(req)
	if err != nil {
		return nil, fmt.Errorf("error sending request: %w", err)
	}
	defer resp.Body.Close()

	var result struct {
		Data struct {
			GetAllPools []Pool `json:"getAllPools"`
		} `json:"data"`
		Errors []struct {
			Message string `json:"message"`
		} `json:"errors,omitempty"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("error parsing response: %w", err)
	}

	if len(result.Errors) > 0 {
		return nil, fmt.Errorf("GraphQL error: %s", result.Errors[0].Message)
	}

	// Accumulate pools from response
	var pools []Pool
	for _, pool := range result.Data.GetAllPools {
		pools = append(pools, Pool{
			ChainName:   pool.ChainName,
			PoolAddress: pool.PoolAddress,
		})
	}

	return pools, nil
}

// GetAllPoolBalances fetches all pool balances
func (c *Client) GetAllPoolBalances() ([]PoolBalance, error) {
	query := `{"query":"query balances{getAllPoolBalances{poolAddress balance}}"}`

	req, err := http.NewRequest("POST", c.baseURL, bytes.NewBuffer([]byte(query)))
	if err != nil {
		return nil, fmt.Errorf("error creating request: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")

	resp, err := c.http.Do(req)
	if err != nil {
		return nil, fmt.Errorf("error sending request: %w", err)
	}
	defer resp.Body.Close()

	var result struct {
		Data struct {
			GetAllPoolBalances []PoolBalance `json:"getAllPoolBalances"`
		} `json:"data"`
		Errors []struct {
			Message string `json:"message"`
		} `json:"errors,omitempty"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("error parsing response: %w", err)
	}

	if len(result.Errors) > 0 {
		return nil, fmt.Errorf("GraphQL error: %s", result.Errors[0].Message)
	}

	return result.Data.GetAllPoolBalances, nil
}

// GetPool fetches pool address for a specific chain
func (c *Client) GetPool(chain string) (string, error) {
	// Reuse existing getPoolAddress method
	return c.getPoolAddress(chain)
}

// getPoolAddress gets the pool address for a given token
func (c *Client) getPoolAddress(token string) (string, error) {
	pools, err := c.GetAllPools()
	if err != nil {
		return "", fmt.Errorf("failed to get pools: %w", err)
	}

	for _, pool := range pools {
		if pool.ChainName == token {
			return pool.PoolAddress, nil
		}
	}

	return "", fmt.Errorf("pool not found for token: %s", token)
}

// Update the prepareEthereumTransaction method
func (c *Client) prepareEthereumTransaction(swap *SwapResponse) error {
	// Get pool address for the token
	fromAddress, err := c.getPoolAddress(swap.SwapResult.ToToken)
	if err != nil {
		return fmt.Errorf("failed to get source pool address: %w", err)
	}

	// Query Ethereum node for current gas price
	client, err := ethclient.Dial(EthereumRPC)
	if err != nil {
		return fmt.Errorf("failed to connect to Ethereum node: %w", err)
	}
	defer client.Close()

	gasPrice, err := client.SuggestGasPrice(context.Background())
	if err != nil {
		return fmt.Errorf("failed to get gas price: %w", err)
	}

	// Get nonce for the from address
	nonce, err := client.PendingNonceAt(context.Background(), common.HexToAddress(fromAddress))
	if err != nil {
		return fmt.Errorf("failed to get nonce: %w", err)
	}

	// Prepare transaction parameters
	swap.TxToSign = &TransactionPrep{
		Chain: "ethereum",
		RawTx: "", // Will be filled by the signer
		ChainParams: ChainParams{
			FromAddress: fromAddress,
			ToAddress:   swap.DestinationAddress,
			Amount:      fmt.Sprintf("%f", swap.SwapResult.ToAmount),
			GasPrice:    gasPrice.String(),
			GasLimit:    21000, // Standard ETH transfer gas limit
			Nonce:       nonce,
		},
	}
	return nil
}

// Update the prepareSolanaTransaction method
func (c *Client) prepareSolanaTransaction(swap *SwapResponse) error {
	// Get pool address for the token
	fromAddress, err := c.getPoolAddress(swap.SwapResult.ToToken)
	if err != nil {
		return fmt.Errorf("failed to get source pool address: %w", err)
	}

	// Query Solana node for recent blockhash
	client := rpc.New(SolanaRPC)
	resp, err := client.GetLatestBlockhash(context.Background(), rpc.CommitmentConfirmed)
	if err != nil {
		return fmt.Errorf("failed to get recent blockhash: %w", err)
	}

	// Prepare transaction parameters
	swap.TxToSign = &TransactionPrep{
		Chain: "solana",
		RawTx: "", // Will be filled by the signer
		ChainParams: ChainParams{
			FromAddress:     fromAddress,
			ToAddress:       swap.DestinationAddress,
			Amount:          fmt.Sprintf("%f", swap.SwapResult.ToAmount),
			RecentBlockhash: resp.Value.Blockhash.String(),
			Lamports:        swap.SwapResult.ToAmount,
		},
	}
	return nil
}

// SignTransaction signs the prepared transaction based on chain type
func (c *Client) SignTransaction(swap *SwapResponse) error {
	if swap.TxToSign == nil {
		return fmt.Errorf("no transaction prepared for signing")
	}

	switch swap.TxToSign.Chain {
	case "ethereum":
		return c.signEthereumTransaction(swap)
	case "solana":
		return c.signSolanaTransaction(swap)
	default:
		return fmt.Errorf("unsupported chain for signing: %s", swap.TxToSign.Chain)
	}
}

func (c *Client) signEthereumTransaction(swap *SwapResponse) error {
	// Get derived Ethereum key instead of environment variable
	if chainKeys == nil || chainKeys.EthereumKey == nil {
		return fmt.Errorf("ethereum private key not initialized")
	}

	// Create the transaction object
	tx := types.NewTransaction(
		swap.TxToSign.ChainParams.Nonce,
		common.HexToAddress(swap.TxToSign.ChainParams.ToAddress),
		func() *big.Int {
			// Convert decimal to integer by multiplying by 10^18 (standard ETH decimals)
			amountFloat, _ := strconv.ParseFloat(swap.TxToSign.ChainParams.Amount, 64)
			amountBigFloat := new(big.Float).SetFloat64(amountFloat)
			multiplier := new(big.Float).SetFloat64(1e18)
			result := new(big.Float).Mul(amountBigFloat, multiplier)

			amountBigInt := new(big.Int)
			result.Int(amountBigInt)
			return amountBigInt
		}(),
		swap.TxToSign.ChainParams.GasLimit,
		func() *big.Int {
			gasPrice, _ := new(big.Int).SetString(swap.TxToSign.ChainParams.GasPrice, 10)
			return gasPrice
		}(),
		nil, // data
	)

	// Get the signer
	chainID := big.NewInt(1337) // mainnet, adjust as needed
	signer := types.NewEIP155Signer(chainID)

	// Sign the transaction
	signedTx, err := types.SignTx(tx, signer, chainKeys.EthereumKey)
	if err != nil {
		return fmt.Errorf("failed to sign transaction: %w", err)
	}

	// Convert to raw bytes
	rawTxBytes, err := signedTx.MarshalBinary()
	if err != nil {
		return fmt.Errorf("failed to encode signed transaction: %w", err)
	}

	// Store the raw signed transaction
	swap.TxToSign.RawTx = hexutil.Encode(rawTxBytes)
	return nil
}

func (c *Client) signSolanaTransaction(swap *SwapResponse) error {
	// Get derived Solana key instead of environment variable
	if chainKeys == nil || chainKeys.SolanaKey == nil {
		return fmt.Errorf("solana private key not initialized")
	}

	from_address, err := solana.PublicKeyFromBase58(swap.TxToSign.ChainParams.FromAddress)
	if err != nil {
		return fmt.Errorf("failed to get from address: %w", err)
	}
	to_address, err := solana.PublicKeyFromBase58(swap.TxToSign.ChainParams.ToAddress)

	if err != nil {
		return fmt.Errorf("failed to get to address: %w", err)
	}

	// Create a new transaction
	tx, err := solana.NewTransaction(
		[]solana.Instruction{
			system.NewTransferInstruction(
				uint64(swap.TxToSign.ChainParams.Lamports),
				from_address,
				to_address,
			).Build(),
		},
		solana.MustHashFromBase58(swap.TxToSign.ChainParams.RecentBlockhash),
	)

	// Sign the transaction
	_, _ = tx.Sign(
		func(key solana.PublicKey) *solana.PrivateKey {
			if chainKeys.SolanaKey.PublicKey().Equals(key) {
				return chainKeys.SolanaKey
			}
			return nil
		},
	)

	// Store the raw signed transaction
	rawTx, err := tx.MarshalBinary()
	if err != nil {
		return fmt.Errorf("failed to serialize signed transaction: %w", err)
	}
	swap.TxToSign.RawTx = base58.Encode(rawTx)

	return nil
}

// SubmitTransaction submits the signed transaction to the appropriate chain
func (c *Client) SubmitTransaction(swap *SwapResponse) error {
	if swap.TxToSign == nil || swap.TxToSign.RawTx == "" {
		return fmt.Errorf("no signed transaction available")
	}

	switch swap.TxToSign.Chain {
	case "ethereum":
		return c.submitEthereumTransaction(swap)
	case "solana":
		return c.submitSolanaTransaction(swap)
	default:
		return fmt.Errorf("unsupported chain for submission: %s", swap.TxToSign.Chain)
	}
}

func (c *Client) submitEthereumTransaction(swap *SwapResponse) error {
	// Connect to Ethereum node
	client, err := ethclient.Dial(EthereumRPC)
	if err != nil {
		return fmt.Errorf("failed to connect to Ethereum node: %w", err)
	}
	defer client.Close()

	// Decode raw transaction
	rawTxBytes, err := hexutil.Decode(swap.TxToSign.RawTx)
	if err != nil {
		return fmt.Errorf("failed to decode raw transaction: %w", err)
	}

	var tx types.Transaction
	if err := tx.UnmarshalBinary(rawTxBytes); err != nil {
		return fmt.Errorf("failed to unmarshal transaction: %w", err)
	}

	// Submit transaction
	if err := client.SendTransaction(context.Background(), &tx); err != nil {
		return fmt.Errorf("failed to submit transaction: %w", err)
	}

	// Update response with transaction hash
	swap.TxHash = tx.Hash().Hex()
	swap.Status = "submitted"

	return nil
}

func (c *Client) submitSolanaTransaction(swap *SwapResponse) error {
	// Create RPC request
	requestBody := map[string]interface{}{
		"jsonrpc": "2.0",
		"id":      1,
		"method":  "sendTransaction",
		"params": []interface{}{
			swap.TxToSign.RawTx,
			map[string]interface{}{
				"encoding": "base58",
			},
		},
	}

	// Submit transaction
	response, err := c.makeRPCRequest(SolanaRPC, requestBody)
	if err != nil {
		return fmt.Errorf("failed to submit transaction: %w", err)
	}

	// Extract transaction signature
	result, ok := response.(map[string]interface{})
	if !ok {
		return fmt.Errorf("invalid response format")
	}

	if errMsg, hasError := result["error"]; hasError {
		return fmt.Errorf("RPC error: %v", errMsg)
	}

	signature, ok := result["result"].(string)
	if !ok {
		return fmt.Errorf("invalid signature format in response")
	}

	// Update response with transaction signature
	swap.TxHash = signature
	swap.Status = "submitted"

	return nil
}

func (c *Client) RequestSolanaAirdrop(address string) (map[string]interface{}, error) {
	// Create RPC client
	client := rpc.New(SolanaRPC)

	// Parse address
	pubKey, err := solana.PublicKeyFromBase58(address)
	if err != nil {
		return nil, fmt.Errorf("invalid Solana address: %w", err)
	}

	// Request airdrop (2 SOL)
	sig, err := client.RequestAirdrop(
		context.Background(),
		pubKey,
		2*solana.LAMPORTS_PER_SOL,
		rpc.CommitmentFinalized,
	)
	if err != nil {
		return nil, fmt.Errorf("failed to request airdrop: %w", err)
	}

	// Wait for confirmation
	// _, err = client.GetConfirmedTransactionWithOpts(context.Background(), sig, &rpc.GetTransactionOpts{
	// 	Commitment: rpc.CommitmentConfirmed,
	// })
	// if err != nil {
	// 	return nil, fmt.Errorf("failed to confirm airdrop: %w", err)
	// }

	return map[string]interface{}{
		"signature": sig.String(),
		"amount":    "2 SOL",
		"address":   address,
	}, nil
}

func (c *Client) RequestEthereumFaucet(address string) (map[string]interface{}, error) {
	// For testnet/local network only
	if !common.IsHexAddress(address) {
		return nil, fmt.Errorf("invalid Ethereum address")
	}

	client, err := ethclient.Dial(EthereumRPC)
	if err != nil {
		return nil, fmt.Errorf("failed to connect to Ethereum node: %w", err)
	}
	defer client.Close()

	// Get the faucet's private key
	if chainKeys == nil || chainKeys.EthereumKey == nil {
		return nil, fmt.Errorf("ethereum faucet key not initialized")
	}

	// Create transaction
	nonce, err := client.PendingNonceAt(context.Background(), crypto.PubkeyToAddress(chainKeys.EthereumKey.PublicKey))
	if err != nil {
		return nil, fmt.Errorf("failed to get nonce: %w", err)
	}

	value := big.NewInt(1000000000000000000) // 1 ETH
	gasLimit := uint64(21000)
	gasPrice, err := client.SuggestGasPrice(context.Background())
	if err != nil {
		return nil, fmt.Errorf("failed to get gas price: %w", err)
	}

	tx := types.NewTransaction(
		nonce,
		common.HexToAddress(address),
		value,
		gasLimit,
		gasPrice,
		nil,
	)

	chainID, err := client.NetworkID(context.Background())
	if err != nil {
		return nil, fmt.Errorf("failed to get chain id: %w", err)
	}

	signedTx, err := types.SignTx(tx, types.NewEIP155Signer(chainID), chainKeys.EthereumKey)
	if err != nil {
		return nil, fmt.Errorf("failed to sign transaction: %w", err)
	}

	err = client.SendTransaction(context.Background(), signedTx)
	if err != nil {
		return nil, fmt.Errorf("failed to send transaction: %w", err)
	}

	return map[string]interface{}{
		"txHash":  signedTx.Hash().String(),
		"amount":  "1 ETH",
		"address": address,
	}, nil
}

// GetSolanaBalance fetches SOL balance for an address
func (c *Client) GetSolanaBalance(address string) (*Balance, error) {
	// Create RPC client
	client := rpc.New(SolanaRPC)

	// Parse address
	pubKey, err := solana.PublicKeyFromBase58(address)
	if err != nil {
		return nil, fmt.Errorf("invalid Solana address: %w", err)
	}

	// Get balance
	balance, err := client.GetBalance(
		context.Background(),
		pubKey,
		rpc.CommitmentFinalized,
	)
	if err != nil {
		return nil, fmt.Errorf("failed to get balance: %w", err)
	}

	// Convert lamports to SOL
	solBalance := float64(balance.Value) / float64(solana.LAMPORTS_PER_SOL)

	return &Balance{
		Address: address,
		Amount:  solBalance,
		Symbol:  "SOL",
	}, nil
}

// GetEthereumBalance fetches ETH balance for an address
func (c *Client) GetEthereumBalance(address string) (*Balance, error) {
	// Validate address
	if !common.IsHexAddress(address) {
		return nil, fmt.Errorf("invalid Ethereum address")
	}

	// Connect to Ethereum node
	client, err := ethclient.Dial(EthereumRPC)
	if err != nil {
		return nil, fmt.Errorf("failed to connect to Ethereum node: %w", err)
	}
	defer client.Close()

	// Get balance
	account := common.HexToAddress(address)
	balance, err := client.BalanceAt(context.Background(), account, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to get balance: %w", err)
	}

	// Convert wei to ETH
	fbalance := new(big.Float)
	fbalance.SetString(balance.String())
	ethValue := new(big.Float).Quo(fbalance, big.NewFloat(1e18))
	amount, _ := ethValue.Float64()

	return &Balance{
		Address: address,
		Amount:  amount,
		Symbol:  "ETH",
	}, nil
}

// Add new functions with amount parameter
func (c *Client) RequestSolanaAirdropWithAmount(address string, amount float64) (map[string]interface{}, error) {
	// Convert amount to lamports (1 SOL = 1e9 lamports)
	lamports := uint64(amount * 1e9)

	client := rpc.New(SolanaRPC)
	pubKey, err := solana.PublicKeyFromBase58(address)
	if err != nil {
		return nil, fmt.Errorf("invalid Solana address: %w", err)
	}

	sig, err := client.RequestAirdrop(
		context.Background(),
		pubKey,
		lamports,
		rpc.CommitmentFinalized,
	)
	if err != nil {
		return nil, fmt.Errorf("failed to request airdrop: %w", err)
	}

	return map[string]interface{}{
		"signature": sig.String(),
		"amount":    fmt.Sprintf("%f SOL", amount),
		"address":   address,
	}, nil
}

func (c *Client) RequestEthereumFaucetWithAmount(address string, amount float64) (map[string]interface{}, error) {
	client, err := ethclient.Dial(EthereumRPC)
	if err != nil {
		return nil, fmt.Errorf("failed to connect to Ethereum node: %w", err)
	}
	defer client.Close()

	// Convert amount to wei (1 ETH = 1e18 wei)
	weiAmount := new(big.Int)
	weiAmount.SetString(fmt.Sprintf("%.0f", amount*1e18), 10)

	// Get the faucet's private key
	privateKey := chainKeys.EthereumKey

	// Get the faucet's nonce
	nonce, err := client.PendingNonceAt(context.Background(), crypto.PubkeyToAddress(privateKey.PublicKey))
	if err != nil {
		return nil, fmt.Errorf("failed to get nonce: %w", err)
	}

	// Create transaction
	gasPrice, err := client.SuggestGasPrice(context.Background())
	if err != nil {
		return nil, fmt.Errorf("failed to get gas price: %w", err)
	}

	tx := types.NewTransaction(
		nonce,
		common.HexToAddress(address),
		weiAmount,
		21000,
		gasPrice,
		nil,
	)

	// Sign transaction
	chainID, err := client.NetworkID(context.Background())
	if err != nil {
		return nil, fmt.Errorf("failed to get chain ID: %w", err)
	}

	signedTx, err := types.SignTx(tx, types.NewEIP155Signer(chainID), privateKey)
	if err != nil {
		return nil, fmt.Errorf("failed to sign transaction: %w", err)
	}

	// Send transaction
	err = client.SendTransaction(context.Background(), signedTx)
	if err != nil {
		return nil, fmt.Errorf("failed to send transaction: %w", err)
	}

	return map[string]interface{}{
		"hash":    signedTx.Hash().String(),
		"amount":  fmt.Sprintf("%f ETH", amount),
		"address": address,
	}, nil
}

// PublishBytecode executes the Linera publish-bytecode command with provided WASM content
func (c *Client) PublishBytecode(contractWasm, serviceWasm []byte) (string, error) {
	Logger.Printf("Publishing bytecode (contract: %d bytes, service: %d bytes)...", len(contractWasm), len(serviceWasm))

	// Create temporary directory for WASM files
	tempDir, err := os.MkdirTemp("", "linera-wasm")
	if err != nil {
		Logger.Printf("Error creating temp directory: %v", err)
		return "", fmt.Errorf("failed to create temp directory: %v", err)
	}
	defer os.RemoveAll(tempDir)

	// Write WASM files directly to temp directory
	contractPath := filepath.Join(tempDir, "solver_contract.wasm")
	servicePath := filepath.Join(tempDir, "solver_service.wasm")

	// Write files using buffered writer for better performance
	if err := writeFileBuffered(contractPath, contractWasm); err != nil {
		Logger.Printf("Error writing contract WASM: %v", err)
		return "", fmt.Errorf("failed to write contract WASM: %v", err)
	}

	if err := writeFileBuffered(servicePath, serviceWasm); err != nil {
		Logger.Printf("Error writing service WASM: %v", err)
		return "", fmt.Errorf("failed to write service WASM: %v", err)
	}

	// Prepare and execute command with environment variables
	cmd := exec.Command("linera", "publish-bytecode", contractPath, servicePath)
	cmd.Env = append(os.Environ(),
		"LINERA_WALLET=/var/folders/3_/ty3nbwgs5cv30xhjxd1s0_3r0000gn/T/.tmpFRJbhX/wallet_0.json",
		"LINERA_STORAGE=rocksdb:/var/folders/3_/ty3nbwgs5cv30xhjxd1s0_3r0000gn/T/.tmpFRJbhX/client_0.db",
		"CHAIN_1=e476187f6ddfeb9d588c7b45d3df334d5501d6499b3f9ad5595cae86cce16a65",
		"OWNER_1=598b7023d32f48573a47acb80ea70781c375fc60a352d8043cf8fcacc5d5b2c9",
		"CHAIN_2=69705f85ac4c9fef6c02b4d83426aaaf05154c645ec1c61665f8e450f0468bc0",
		"OWNER_2=5dcc4b83f44bfd28086560c5c4872cfd6979dee316d1b6b3ee8da038199ca0a3",
	)

	// Get the output using pipe for better performance with large outputs
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return "", fmt.Errorf("failed to create stdout pipe: %v", err)
	}

	if err := cmd.Start(); err != nil {
		return "", fmt.Errorf("failed to start command: %v", err)
	}

	// Read output using scanner for better memory efficiency
	var outputBuilder strings.Builder
	scanner := bufio.NewScanner(stdout)
	for scanner.Scan() {
		outputBuilder.WriteString(scanner.Text())
		outputBuilder.WriteString("\n")
	}

	if err := cmd.Wait(); err != nil {
		return "", fmt.Errorf("command failed: %v", err)
	}

	if err := scanner.Err(); err != nil {
		return "", fmt.Errorf("error reading command output: %v", err)
	}

	// Parse the output to get the bytecode ID
	outputStr := outputBuilder.String()
	parts := strings.Split(outputStr, "=")
	if len(parts) != 2 {
		Logger.Printf("Unexpected output format: %s", outputStr)
		return "", fmt.Errorf("unexpected output format: %s", outputStr)
	}

	bytecodeID := strings.TrimSpace(parts[1])
	Logger.Printf("Successfully published bytecode with ID: %s", bytecodeID)

	return bytecodeID, nil
}

// writeFileBuffered writes data to a file using a buffered writer for better performance
func writeFileBuffered(filepath string, data []byte) error {
	file, err := os.OpenFile(filepath, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0644)
	if err != nil {
		return err
	}
	defer file.Close()

	writer := bufio.NewWriter(file)
	if _, err := writer.Write(data); err != nil {
		return err
	}

	return writer.Flush()
}

// PublishBytecodeFromFiles executes the Linera publish-bytecode command with provided file paths
func (c *Client) PublishBytecodeFromFiles(contractPath, servicePath string) (string, error) {
	Logger.Printf("Publishing bytecode from files...")

	// Prepare and execute command with environment variables
	cmd := exec.Command("linera", "publish-bytecode", contractPath, servicePath)
	cmd.Env = append(os.Environ(),
		"LINERA_WALLET=/var/folders/3_/ty3nbwgs5cv30xhjxd1s0_3r0000gn/T/.tmpFRJbhX/wallet_0.json",
		"LINERA_STORAGE=rocksdb:/var/folders/3_/ty3nbwgs5cv30xhjxd1s0_3r0000gn/T/.tmpFRJbhX/client_0.db",
		"CHAIN_1=e476187f6ddfeb9d588c7b45d3df334d5501d6499b3f9ad5595cae86cce16a65",
		"OWNER_1=598b7023d32f48573a47acb80ea70781c375fc60a352d8043cf8fcacc5d5b2c9",
		"CHAIN_2=69705f85ac4c9fef6c02b4d83426aaaf05154c645ec1c61665f8e450f0468bc0",
		"OWNER_2=5dcc4b83f44bfd28086560c5c4872cfd6979dee316d1b6b3ee8da038199ca0a3",
	)

	// Get the output using pipe for better performance
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return "", fmt.Errorf("failed to create stdout pipe: %v", err)
	}

	if err := cmd.Start(); err != nil {
		return "", fmt.Errorf("failed to start command: %v", err)
	}

	// Read output using scanner for better memory efficiency
	var outputBuilder strings.Builder
	scanner := bufio.NewScanner(stdout)
	for scanner.Scan() {
		outputBuilder.WriteString(scanner.Text())
		outputBuilder.WriteString("\n")
	}

	if err := cmd.Wait(); err != nil {
		return "", fmt.Errorf("command failed: %v", err)
	}

	if err := scanner.Err(); err != nil {
		return "", fmt.Errorf("error reading command output: %v", err)
	}

	// Parse the output to get the bytecode ID
	outputStr := strings.TrimSpace(outputBuilder.String())

	Logger.Printf("Successfully published bytecode with ID: %s", outputStr)
	return outputStr, nil
}

// CreateApplication executes the Linera create-application command with the provided bytecode ID
func (c *Client) CreateApplication(bytecodeID string) (string, error) {
	Logger.Printf("Creating application with bytecode ID: %s", bytecodeID)

	// Prepare the command
	cmd := exec.Command("linera", "create-application", bytecodeID)
	cmd.Env = append(os.Environ(),
		"LINERA_WALLET=/var/folders/3_/ty3nbwgs5cv30xhjxd1s0_3r0000gn/T/.tmpFRJbhX/wallet_0.json",
		"LINERA_STORAGE=rocksdb:/var/folders/3_/ty3nbwgs5cv30xhjxd1s0_3r0000gn/T/.tmpFRJbhX/client_0.db",
		"CHAIN_1=e476187f6ddfeb9d588c7b45d3df334d5501d6499b3f9ad5595cae86cce16a65",
		"OWNER_1=598b7023d32f48573a47acb80ea70781c375fc60a352d8043cf8fcacc5d5b2c9",
		"CHAIN_2=69705f85ac4c9fef6c02b4d83426aaaf05154c645ec1c61665f8e450f0468bc0",
		"OWNER_2=5dcc4b83f44bfd28086560c5c4872cfd6979dee316d1b6b3ee8da038199ca0a3",
	)

	// Get the output using pipe for better performance
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return "", fmt.Errorf("failed to create stdout pipe: %v", err)
	}

	if err := cmd.Start(); err != nil {
		return "", fmt.Errorf("failed to start command: %v", err)
	}

	// Read output using scanner for better memory efficiency
	var outputBuilder strings.Builder
	scanner := bufio.NewScanner(stdout)
	for scanner.Scan() {
		outputBuilder.WriteString(scanner.Text())
		outputBuilder.WriteString("\n")
	}

	if err := cmd.Wait(); err != nil {
		return "", fmt.Errorf("command failed: %v", err)
	}

	if err := scanner.Err(); err != nil {
		return "", fmt.Errorf("error reading command output: %v", err)
	}

	// Parse the output to get the application ID
	outputStr := strings.TrimSpace(outputBuilder.String())
	Logger.Printf("Successfully created application with ID: %s", outputStr)

	return outputStr, nil
}
