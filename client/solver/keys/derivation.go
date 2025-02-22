package keys

import (
	"crypto/ecdsa"
	"crypto/ed25519"
	"crypto/sha512"
	"fmt"

	"github.com/gagliardetto/solana-go"
	hdwallet "github.com/miguelmota/go-ethereum-hdwallet"
	"github.com/tyler-smith/go-bip39"
)

type ChainKeys struct {
	EthereumKey *ecdsa.PrivateKey
	SolanaKey   *solana.PrivateKey
}

// DeriveKeysFromSeedPhrase derives both Ethereum and Solana private keys from a seed phrase
func DeriveKeysFromSeedPhrase(seedPhrase string) (*ChainKeys, error) {
	// Validate seed phrase
	if !bip39.IsMnemonicValid(seedPhrase) {
		return nil, fmt.Errorf("invalid seed phrase")
	}

	// Generate seed from mnemonic
	seed := bip39.NewSeed(seedPhrase, "")

	// Derive Ethereum key
	ethKey, err := deriveEthereumKey(seed)
	if err != nil {
		return nil, fmt.Errorf("failed to derive Ethereum key: %w", err)
	}

	// Derive Solana key
	solKey, err := deriveSolanaKey(seed)
	if err != nil {
		return nil, fmt.Errorf("failed to derive Solana key: %w", err)
	}

	return &ChainKeys{
		EthereumKey: ethKey,
		SolanaKey:   solKey,
	}, nil
}

// deriveEthereumKey derives an Ethereum private key from a seed using BIP44
func deriveEthereumKey(seed []byte) (*ecdsa.PrivateKey, error) {
	wallet, err := hdwallet.NewFromSeed(seed)
	if err != nil {
		return nil, err
	}

	// BIP44 path for Ethereum: m/44'/60'/0'/0/0
	path := hdwallet.MustParseDerivationPath("m/44'/60'/0'/0/0")
	account, err := wallet.Derive(path, true)
	if err != nil {
		return nil, err
	}

	privateKey, err := wallet.PrivateKey(account)
	if err != nil {
		return nil, err
	}

	return privateKey, nil
}

// deriveSolanaKey derives a Solana private key from a seed using ed25519
func deriveSolanaKey(seed []byte) (*solana.PrivateKey, error) {
	// Use SHA512 to derive ed25519 key from seed
	hash := sha512.Sum512(seed)

	// Generate ed25519 key pair
	privateKey := ed25519.NewKeyFromSeed(hash[:32])

	// Convert to Solana private key
	solPrivKey := solana.PrivateKey(privateKey)

	return &solPrivKey, nil
}
