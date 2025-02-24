package main

import (
	"bufio"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"log"
	"math/big"
	"net/http"
	"os"
	"strconv"
	"strings"
	"testing"

	"github.com/linera-protocol/examples/universal-solver/client/solver"
)

var (
	solverClient *solver.Client
	SolanaRPC    string
	EthereumRPC  string
	chainToToken = map[string]string{
		"ethereum": "ETH",
		"solana":   "SOL",
	}
)

func init() {
	initFlags()
	// Initialize the solver logger
	solver.InitLogger()
}

func initFlags() {
	// Define command line flags
	solverURL := flag.String("solver-url", getEnvOrDefault("SOLVER_URL", "http://localhost:8080/"), "Universal Solver service URL")
	solanaRPCURL := flag.String("solana-url", getEnvOrDefault("SOLANA_RPC", "http://localhost:8899"), "Solana RPC endpoint")
	ethereumRPCURL := flag.String("ethereum-url", getEnvOrDefault("ETHEREUM_RPC", "http://localhost:8545"), "Ethereum RPC endpoint")
	seedPhrase := flag.String("seed-phrase", "", "Seed phrase for deriving chain keys (required)")

	// Only parse flags if not running tests
	if !testing.Testing() {
		flag.Parse()

		// Validate required seed phrase
		if *seedPhrase == "" {
			fmt.Println("Usage:")
			fmt.Println("  -solver-url string")
			fmt.Println("        Universal Solver service URL (default: http://localhost:8080/)")
			fmt.Println("  -solana-url string")
			fmt.Println("        Solana RPC endpoint (default: http://localhost:8899)")
			fmt.Println("  -ethereum-url string")
			fmt.Println("        Ethereum RPC endpoint (default: http://localhost:8545)")
			fmt.Println("  -seed-phrase string")
			fmt.Println("        Seed phrase for deriving chain keys (required)")
			os.Exit(1)
		}
	}

	// Initialize solver client with provided URL
	solverClient = solver.NewClient(*solverURL)

	// Initialize RPC endpoints
	solver.InitRPCEndpoints(*ethereumRPCURL, *solanaRPCURL)

	// Initialize keys with seed phrase
	if err := solver.InitKeys(*seedPhrase); err != nil {
		log.Fatalf("Failed to initialize keys: %v", err)
	}

	// Log configuration (without exposing seed phrase)
	solver.Logger.Printf("Initialized with:")
	solver.Logger.Printf("  Solver URL: %s", *solverURL)
	solver.Logger.Printf("  Solana RPC: %s", *solanaRPCURL)
	solver.Logger.Printf("  Ethereum RPC: %s", *ethereumRPCURL)
	solver.Logger.Printf("  Keys: Initialized successfully")
}

func getEnvOrDefault(key, defaultValue string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return defaultValue
}

// Add CORS middleware
func corsMiddleware(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		// Set CORS headers
		w.Header().Set("Access-Control-Allow-Origin", "*")
		w.Header().Set("Access-Control-Allow-Methods", "GET, POST, OPTIONS")
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type")

		// Handle preflight requests
		if r.Method == "OPTIONS" {
			w.WriteHeader(http.StatusOK)
			return
		}

		next(w, r)
	}
}

func main() {
	// Define routes with CORS middleware
	http.HandleFunc("/post_tx_hash", corsMiddleware(handlePostTxHash))
	http.HandleFunc("/faucet", corsMiddleware(handleFaucet))
	http.HandleFunc("/get_pool_address", corsMiddleware(handleGetPoolAddress))
	http.HandleFunc("/fetch_balance", corsMiddleware(handleFetchBalance))
	http.HandleFunc("/quote_swap", corsMiddleware(handleQuoteSwap))
	http.HandleFunc("/deploy_bytecode", corsMiddleware(handleDeployBytecode))
	http.HandleFunc("/create_application", corsMiddleware(handleCreateApplication))

	// Start server
	port := getEnvOrDefault("PORT", "3001")
	log.Printf("Server starting on :%s", port)
	if err := http.ListenAndServe(":"+port, nil); err != nil {
		log.Fatalf("Error starting server: %v", err)
	}
}

func handlePostTxHash(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// Get parameters from query params
	txHash := r.URL.Query().Get("txHash")
	chain := r.URL.Query().Get("chain")
	toToken := r.URL.Query().Get("toToken")
	destinationAddress := r.URL.Query().Get("destinationAddress")

	// Validate required parameters
	if txHash == "" {
		http.Error(w, "txHash parameter is required", http.StatusBadRequest)
		return
	}

	if chain == "" {
		http.Error(w, "chain parameter is required", http.StatusBadRequest)
		return
	}

	var (
		tx  interface{}
		err error
	)

	// Get transaction details based on chain
	switch chain {
	case "solana":
		tx, err = solverClient.GetSolanaTransaction(SolanaRPC, txHash)
	case "ethereum":
		tx, err = solverClient.GetEthereumTransaction(EthereumRPC, txHash)
	default:
		http.Error(w, "Invalid chain parameter. Must be 'solana' or 'ethereum'", http.StatusBadRequest)
		return
	}

	if err != nil {
		http.Error(w, "Error getting transaction: "+err.Error(), http.StatusInternalServerError)
		return
	}

	response := map[string]interface{}{
		"status": "success",
		"chain":  chain,
		"data":   tx,
	}

	// If toToken and destinationAddress are provided, execute swap
	if toToken != "" && destinationAddress != "" {
		// Get the from token based on chain
		fromToken, err := getTokenForChain(chain)
		if err != nil {
			http.Error(w, "Error getting token for chain: "+err.Error(), http.StatusInternalServerError)
			return
		}

		// Extract amount from transaction
		amount, err := extractAmountFromTx(tx)
		if err != nil {
			http.Error(w, "Error extracting amount from transaction: "+err.Error(), http.StatusInternalServerError)
			return
		}

		// Execute swap with correct fromToken
		swapResponse, err := solverClient.ExecuteSwap(fromToken, toToken, float64(amount), destinationAddress)
		if err != nil {
			http.Error(w, "Error executing swap: "+err.Error(), http.StatusInternalServerError)
			return
		}

		response["swap_result"] = swapResponse
	}

	// Return response
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(response)
}

// Helper function to extract amount from transaction
func extractAmountFromTx(tx interface{}) (uint64, error) {
	switch v := tx.(type) {
	case map[string]interface{}:
		// For Ethereum
		if value, ok := v["value"].(string); ok {
			// Parse decimal string to big.Int
			bigValue := new(big.Int)
			if _, success := bigValue.SetString(value, 10); !success {
				return 0, fmt.Errorf("failed to parse decimal value: %s", value)
			}
			// Convert from wei to ETH (divide by 10^18) and check if result fits uint64
			weiPerEth := new(big.Int).Exp(big.NewInt(10), big.NewInt(18), nil)
			ethValue := new(big.Int).Div(bigValue, weiPerEth)
			if !ethValue.IsUint64() {
				return 0, fmt.Errorf("converted ETH value exceeds uint64 range: %s", ethValue.String())
			}
			return ethValue.Uint64(), nil
		}
		// For Solana
		if result, ok := v["result"].(map[string]interface{}); ok {
			meta := result
			if meta, ok := meta["meta"].(map[string]interface{}); ok {
				if preBalances, ok := meta["preBalances"].([]interface{}); ok && len(preBalances) > 0 {
					if postBalances, ok := meta["postBalances"].([]interface{}); ok && len(postBalances) > 0 {
						// Get the difference between pre and post balances of sender
						preBalance := uint64(preBalances[0].(float64))
						postBalance := uint64(postBalances[0].(float64))
						if preBalance > postBalance {
							// Convert from lamports to SOL (divide by 10^9)
							lamports := preBalance - postBalance
							solValue := float64(lamports) / 1e9
							if solValue > float64(^uint64(0)) {
								return 0, fmt.Errorf("converted SOL value exceeds uint64 range: %f", solValue)
							}
							return uint64(solValue), nil
						}
					}
				}
			}
		}
	}
	return 0, fmt.Errorf("could not extract amount from transaction")
}

func getTokenForChain(chain string) (string, error) {
	token, ok := chainToToken[chain]
	if !ok {
		return "", fmt.Errorf("unsupported chain: %s", chain)
	}
	return token, nil
}

// Update handleFaucet to accept amount parameter
func handleFaucet(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// Get parameters
	chain := r.URL.Query().Get("chain")
	if chain == "" {
		http.Error(w, "chain parameter is required", http.StatusBadRequest)
		return
	}

	address := r.URL.Query().Get("address")
	if address == "" {
		http.Error(w, "address parameter is required", http.StatusBadRequest)
		return
	}

	// Get optional amount parameter
	amount := r.URL.Query().Get("amount")
	var amountFloat float64
	var err error
	if amount != "" {
		amountFloat, err = strconv.ParseFloat(amount, 64)
		if err != nil {
			http.Error(w, "Invalid amount value", http.StatusBadRequest)
			return
		}
	}

	var result map[string]interface{}

	switch chain {
	case "solana":
		if amount == "" {
			result, err = solverClient.RequestSolanaAirdrop(address)
		} else {
			result, err = solverClient.RequestSolanaAirdropWithAmount(address, amountFloat)
		}
	case "ethereum":
		if amount == "" {
			result, err = solverClient.RequestEthereumFaucet(address)
		} else {
			result, err = solverClient.RequestEthereumFaucetWithAmount(address, amountFloat)
		}
	default:
		http.Error(w, "Invalid chain parameter. Must be 'solana' or 'ethereum'", http.StatusBadRequest)
		return
	}

	if err != nil {
		http.Error(w, fmt.Sprintf("Error requesting faucet: %v", err), http.StatusInternalServerError)
		return
	}

	// Return response
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"status": "success",
		"chain":  chain,
		"data":   result,
	})
}

// Add new handler function
func handleGetPoolAddress(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// Get chain parameter
	chain := r.URL.Query().Get("chain")
	if chain == "" {
		http.Error(w, "chain parameter is required", http.StatusBadRequest)
		return
	}

	// Get pool address for the chain
	poolAddress, err := solverClient.GetPool(chain)
	if err != nil {
		if err.Error() == fmt.Sprintf("pool not found for token: %s", chain) {
			http.Error(w, fmt.Sprintf("No pool found for chain: %s", chain), http.StatusNotFound)
			return
		}
		http.Error(w, fmt.Sprintf("Error fetching pool: %v", err), http.StatusInternalServerError)
		return
	}

	// Return response
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"status": "success",
		"chain":  chain,
		"data": map[string]interface{}{
			"address": poolAddress,
		},
	})
}

func handleFetchBalance(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// Get parameters
	chain := r.URL.Query().Get("chain")
	if chain == "" {
		http.Error(w, "chain parameter is required", http.StatusBadRequest)
		return
	}

	address := r.URL.Query().Get("address")
	if address == "" {
		http.Error(w, "address parameter is required", http.StatusBadRequest)
		return
	}

	// Get balance based on chain
	var balance *solver.Balance
	var err error

	switch chain {
	case "solana":
		balance, err = solverClient.GetSolanaBalance(address)
	case "ethereum":
		balance, err = solverClient.GetEthereumBalance(address)
	default:
		http.Error(w, "Invalid chain parameter. Must be 'solana' or 'ethereum'", http.StatusBadRequest)
		return
	}

	if err != nil {
		http.Error(w, fmt.Sprintf("Error fetching balance: %v", err), http.StatusInternalServerError)
		return
	}

	// Return response
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"status": "success",
		"chain":  chain,
		"data":   balance,
	})
}

func handleQuoteSwap(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// Get parameters
	fromChain := r.URL.Query().Get("fromChain")
	if fromChain == "" {
		http.Error(w, "fromChain parameter is required", http.StatusBadRequest)
		return
	}

	toChain := r.URL.Query().Get("toChain")
	if toChain == "" {
		http.Error(w, "toChain parameter is required", http.StatusBadRequest)
		return
	}

	fromAmount := r.URL.Query().Get("fromAmount")
	if fromAmount == "" {
		http.Error(w, "fromAmount parameter is required", http.StatusBadRequest)
		return
	}

	// Convert amount to float64
	amount, err := strconv.ParseFloat(fromAmount, 64)
	if err != nil {
		http.Error(w, "Invalid fromAmount value", http.StatusBadRequest)
		return
	}

	// Get quote using calculate swap
	quote, err := solverClient.CalculateSwap(fromChain, toChain, amount)
	if err != nil {
		http.Error(w, fmt.Sprintf("Error calculating swap: %v", err), http.StatusInternalServerError)
		return
	}

	// Return response
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"status": "success",
		"data": map[string]interface{}{
			"fromChain":    fromChain,
			"toChain":      toChain,
			"fromAmount":   amount,
			"toAmount":     quote.ToAmount,
			"exchangeRate": quote.ExchangeRate,
		},
	})
}

// StreamingRequest represents a streaming request with size information

func handleDeployBytecode(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// Create buffered reader for the request body
	bodyReader := bufio.NewReaderSize(r.Body, 1024*1024) // 1MB buffer

	// Read the contract size header
	contractSizeStr, err := bodyReader.ReadString('|')
	if err != nil {
		http.Error(w, fmt.Sprintf("Error reading contract size: %v", err), http.StatusBadRequest)
		return
	}
	contractSize, err := strconv.ParseInt(strings.TrimSuffix(contractSizeStr, "|"), 10, 64)
	if err != nil {
		http.Error(w, fmt.Sprintf("Invalid contract size: %v", err), http.StatusBadRequest)
		return
	}

	// Read the service size header
	serviceSizeStr, err := bodyReader.ReadString('|')
	if err != nil {
		http.Error(w, fmt.Sprintf("Error reading service size: %v", err), http.StatusBadRequest)
		return
	}
	serviceSize, err := strconv.ParseInt(strings.TrimSuffix(serviceSizeStr, "|"), 10, 64)
	if err != nil {
		http.Error(w, fmt.Sprintf("Invalid service size: %v", err), http.StatusBadRequest)
		return
	}

	solver.Logger.Printf("Receiving WASM files - Contract: %d bytes, Service: %d bytes",
		contractSize, serviceSize)

	// Create temporary files with buffered writers
	contractFile, err := os.CreateTemp("/Users/luffybhaagi/RustroverProjects/linera-protocol-jvff/examples/universal-solver", "contract.wasm")
	if err != nil {
		http.Error(w, fmt.Sprintf("Error creating temp file: %v", err), http.StatusInternalServerError)
		return
	}
	defer os.Remove(contractFile.Name())
	defer contractFile.Close()

	serviceFile, err := os.CreateTemp("/Users/luffybhaagi/RustroverProjects/linera-protocol-jvff/examples/universal-solver", "service.wasm")
	if err != nil {
		http.Error(w, fmt.Sprintf("Error creating temp file: %v", err), http.StatusInternalServerError)
		return
	}
	defer os.Remove(serviceFile.Name())
	defer serviceFile.Close()

	// Create buffered writers
	contractWriter := bufio.NewWriterSize(contractFile, 1024*1024) // 1MB buffer
	serviceWriter := bufio.NewWriterSize(serviceFile, 1024*1024)   // 1MB buffer

	// Copy contract WASM with progress tracking
	contractWritten, err := io.CopyN(contractWriter, bodyReader, contractSize)
	if err != nil {
		http.Error(w, fmt.Sprintf("Error writing contract WASM: %v", err), http.StatusInternalServerError)
		return
	}
	if err := contractWriter.Flush(); err != nil {
		http.Error(w, fmt.Sprintf("Error flushing contract WASM: %v", err), http.StatusInternalServerError)
		return
	}

	// Copy service WASM with progress tracking
	serviceWritten, err := io.CopyN(serviceWriter, bodyReader, serviceSize)
	if err != nil {
		http.Error(w, fmt.Sprintf("Error writing service WASM: %v", err), http.StatusInternalServerError)
		return
	}
	if err := serviceWriter.Flush(); err != nil {
		http.Error(w, fmt.Sprintf("Error flushing service WASM: %v", err), http.StatusInternalServerError)
		return
	}

	// Verify sizes
	if contractWritten != contractSize || serviceWritten != serviceSize {
		http.Error(w, "WASM file size mismatch", http.StatusBadRequest)
		return
	}

	solver.Logger.Printf("Successfully received WASM files - Contract: %d bytes, Service: %d bytes",
		contractWritten, serviceWritten)

	// Execute the publish bytecode command
	bytecodeID, err := solverClient.PublishBytecodeFromFiles(contractFile.Name(), serviceFile.Name())
	if err != nil {
		http.Error(w, fmt.Sprintf("Error publishing bytecode: %v", err), http.StatusInternalServerError)
		return
	}

	// Return response
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"status": "success",
		"data": map[string]interface{}{
			"bytecodeId":   bytecodeID,
			"contractSize": contractWritten,
			"serviceSize":  serviceWritten,
		},
	})
}

func handleCreateApplication(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// Get bytecode ID from request body
	var req struct {
		BytecodeID string `json:"bytecodeId"`
	}

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, fmt.Sprintf("Error parsing request body: %v", err), http.StatusBadRequest)
		return
	}

	// Validate bytecode ID
	if req.BytecodeID == "" {
		http.Error(w, "bytecodeId is required", http.StatusBadRequest)
		return
	}

	// Create the application
	applicationID, err := solverClient.CreateApplication(req.BytecodeID)
	if err != nil {
		http.Error(w, fmt.Sprintf("Error creating application: %v", err), http.StatusInternalServerError)
		return
	}

	// Return response
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"status": "success",
		"data": map[string]interface{}{
			"applicationId": applicationID,
		},
	})
}
