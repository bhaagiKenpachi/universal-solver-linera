package solver

type SolverFile struct {
	SolverFileId string `json:"solverFileId"`
	Owner        string `json:"owner"`
	Name         string `json:"name"`
	Payload      []byte `json:"payload"`
}

type GraphQLResponse struct {
	Data struct {
		GetFileSolverApp SolverFile `json:"getFileSolverApp"`
	} `json:"data"`
}

// Transaction represents an Ethereum transaction
type Transaction struct {
	Hash             string `json:"hash"`
	BlockHash        string `json:"blockHash"`
	BlockNumber      string `json:"blockNumber"`
	From             string `json:"from"`
	To               string `json:"to"`
	Value            string `json:"value"`
	GasPrice         string `json:"gasPrice"`
	Gas              string `json:"gas"`
	Nonce            string `json:"nonce"`
	Input            string `json:"input"`
	TransactionIndex string `json:"transactionIndex"`
	V                string `json:"v"`
	R                string `json:"r"`
	S                string `json:"s"`
}

type SwapResult struct {
	FromToken    string  `json:"from_token"`
	ToToken      string  `json:"to_token"`
	FromAmount   float64 `json:"from_amount"`
	ToAmount     float64 `json:"to_amount"`
	ExchangeRate float64 `json:"exchange_rate"`
}

type TransactionPrep struct {
	Chain       string      `json:"chain"`
	RawTx       string      `json:"raw_tx"`
	ChainParams ChainParams `json:"chain_params"`
}

type ChainParams struct {
	// Common params
	FromAddress string `json:"from_address"`
	ToAddress   string `json:"to_address"`
	Amount      string `json:"amount"`

	// Ethereum specific
	GasPrice string `json:"gas_price,omitempty"`
	GasLimit uint64 `json:"gas_limit,omitempty"`
	Nonce    uint64 `json:"nonce,omitempty"`

	// Solana specific
	RecentBlockhash string  `json:"recent_blockhash,omitempty"`
	Lamports        float64 `json:"lamports,omitempty"`
}

type SwapResponse struct {
	TxHash             string           `json:"tx_hash"`
	SwapResult         SwapResult       `json:"swap_result"`
	Status             string           `json:"status"`
	TxToSign           *TransactionPrep `json:"tx_to_sign,omitempty"`
	DestinationAddress string           `json:"destination_address"`
}

type Pool struct {
	ChainName   string `json:"chainName"`
	PoolAddress string `json:"poolAddress"`
}

type PoolBalance struct {
	PoolAddress string  `json:"pool_address"`
	Balance     float64 `json:"balance"`
}

// Add new type for balance responses
type Balance struct {
	Address string  `json:"address"`
	Amount  float64 `json:"amount"`
	Symbol  string  `json:"symbol"`
}
