package node

import (
	"crypto/ed25519"
	"crypto/hmac"
	"crypto/sha256"
	"crypto/sha512"
	"encoding/binary"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"strings"

	"github.com/tyler-smith/go-bip39"
)

// ── BIP-39 Mnemonic ──

// NewMnemonic generates a new 24-word BIP-39 mnemonic (256-bit entropy).
func NewMnemonic() (string, error) {
	entropy, err := bip39.NewEntropy(256)
	if err != nil {
		return "", fmt.Errorf("failed to generate entropy: %w", err)
	}
	mnemonic, err := bip39.NewMnemonic(entropy)
	if err != nil {
		return "", fmt.Errorf("failed to generate mnemonic: %w", err)
	}
	return mnemonic, nil
}

// MnemonicToSeed converts a 24-word mnemonic back to 32-byte seed (entropy).
// Note: we use raw entropy, NOT BIP-39 seed (which applies PBKDF2 with passphrase).
// This keeps our seed = Ed25519 seed = 32 bytes, matching existing identity format.
func MnemonicToSeed(mnemonic string) ([]byte, error) {
	if !bip39.IsMnemonicValid(mnemonic) {
		return nil, fmt.Errorf("invalid mnemonic")
	}
	entropy, err := bip39.EntropyFromMnemonic(mnemonic)
	if err != nil {
		return nil, fmt.Errorf("failed to decode mnemonic: %w", err)
	}
	return entropy, nil
}

// SeedToMnemonic converts a 32-byte seed (entropy) to a 24-word mnemonic.
func SeedToMnemonic(seed []byte) (string, error) {
	mnemonic, err := bip39.NewMnemonic(seed)
	if err != nil {
		return "", fmt.Errorf("failed to encode mnemonic: %w", err)
	}
	return mnemonic, nil
}

// ── SLIP-0010 HD Key Derivation for Ed25519 ──
//
// Reference: https://github.com/satoshilabs/slips/blob/master/slip-0010.md
// Ed25519 only supports hardened derivation (index >= 0x80000000).
//
// BIP-44 path for StarClaw: m/44'/9001'/account'/change/index
//   44'    = BIP-44 purpose
//   9001'  = StarClaw coin type
//   account' = account index (0, 1, 2, ...)
//   change = 0 (external/receiving) or 1 (internal/change)
//   index  = address index

const (
	CoinType       = 9001 // StarClaw coin type
	HardenedOffset = 0x80000000
)

// ExtendedKey represents an HD extended key (private + chain code).
type ExtendedKey struct {
	PrivateKey ed25519.PrivateKey
	PublicKey  ed25519.PublicKey
	ChainCode  []byte
	Depth      uint8
	Path       string
}

// MasterKeyFromSeed derives the master extended key from a seed using SLIP-0010.
// seed should be 32 bytes (from BIP-39 entropy or Ed25519 seed).
func MasterKeyFromSeed(seed []byte) *ExtendedKey {
	// SLIP-0010: HMAC-SHA512("ed25519 seed", seed)
	mac := hmac.New(sha512.New, []byte("ed25519 seed"))
	mac.Write(seed)
	I := mac.Sum(nil)

	IL := I[:32] // private key seed
	IR := I[32:] // chain code

	privateKey := ed25519.NewKeyFromSeed(IL)
	publicKey := privateKey.Public().(ed25519.PublicKey)

	return &ExtendedKey{
		PrivateKey: privateKey,
		PublicKey:  publicKey,
		ChainCode:  IR,
		Depth:      0,
		Path:       "m",
	}
}

// DeriveHardened derives a hardened child key at the given index.
// Ed25519 HD derivation only supports hardened children (SLIP-0010).
func (k *ExtendedKey) DeriveHardened(index uint32) *ExtendedKey {
	// data = 0x00 || private_key_seed(32) || ser32(index + 0x80000000)
	data := make([]byte, 1+32+4)
	data[0] = 0x00
	copy(data[1:33], k.PrivateKey.Seed())
	binary.BigEndian.PutUint32(data[33:], index+HardenedOffset)

	mac := hmac.New(sha512.New, k.ChainCode)
	mac.Write(data)
	I := mac.Sum(nil)

	IL := I[:32]
	IR := I[32:]

	childPriv := ed25519.NewKeyFromSeed(IL)
	childPub := childPriv.Public().(ed25519.PublicKey)

	return &ExtendedKey{
		PrivateKey: childPriv,
		PublicKey:  childPub,
		ChainCode:  IR,
		Depth:      k.Depth + 1,
		Path:       fmt.Sprintf("%s/%d'", k.Path, index),
	}
}

// DeriveBIP44 derives a key using BIP-44 path: m/44'/9001'/account'/change/index
// All levels are hardened for Ed25519 (SLIP-0010 requirement).
func (k *ExtendedKey) DeriveBIP44(account, change, index uint32) *ExtendedKey {
	purpose := k.DeriveHardened(44)       // m/44'
	coin := purpose.DeriveHardened(CoinType) // m/44'/9001'
	acct := coin.DeriveHardened(account)     // m/44'/9001'/account'
	chg := acct.DeriveHardened(change)       // m/44'/9001'/account'/change'
	addr := chg.DeriveHardened(index)        // m/44'/9001'/account'/change'/index'

	addr.Path = fmt.Sprintf("m/44'/9001'/%d'/%d'/%d'", account, change, index)
	return addr
}

// NodeID returns the claw: address for this key.
func (k *ExtendedKey) NodeID() string {
	return deriveNodeID(k.PublicKey)
}

// Fingerprint returns the human-readable fingerprint for this key.
func (k *ExtendedKey) Fingerprint() string {
	hash := sha256.Sum256(k.PublicKey)
	fp := hex.EncodeToString(hash[:])
	result := ""
	for i := 0; i < 32 && i < len(fp); i += 2 {
		if result != "" {
			result += ":"
		}
		result += fp[i : i+2]
	}
	return result
}

// ── Wallet Model ──
//
// A Wallet holds the master key and can derive child keys.
// Cold wallet: stores master mnemonic offline, only signs critical transactions.
// Hot wallet: uses a derived key (m/44'/9001'/0'/0'/0') for everyday operations.

// Wallet represents a StarClaw HD wallet.
type Wallet struct {
	Mnemonic  string       `json:"-"`          // 24-word mnemonic (cold storage only)
	MasterKey *ExtendedKey `json:"-"`          // master extended key
	HotKey    *ExtendedKey `json:"-"`          // derived hot key for daily use
	NodeID    string       `json:"node_id"`    // master address (cold wallet address)
	HotNodeID string       `json:"hot_node_id"` // hot wallet address
}

// NewWallet creates a new wallet from a fresh mnemonic.
func NewWallet() (*Wallet, error) {
	mnemonic, err := NewMnemonic()
	if err != nil {
		return nil, err
	}
	return WalletFromMnemonic(mnemonic)
}

// WalletFromMnemonic restores a wallet from a 24-word mnemonic.
func WalletFromMnemonic(mnemonic string) (*Wallet, error) {
	seed, err := MnemonicToSeed(mnemonic)
	if err != nil {
		return nil, err
	}
	return WalletFromSeed(seed, mnemonic), nil
}

// WalletFromSeed creates a wallet from a raw seed (32 bytes).
func WalletFromSeed(seed []byte, mnemonic string) *Wallet {
	master := MasterKeyFromSeed(seed)
	// Hot key: m/44'/9001'/0'/0'/0' (first external address of first account)
	hot := master.DeriveBIP44(0, 0, 0)

	return &Wallet{
		Mnemonic:  mnemonic,
		MasterKey: master,
		HotKey:    hot,
		NodeID:    master.NodeID(),
		HotNodeID: hot.NodeID(),
	}
}

// DeriveAddress derives a new address at the given account/index.
// change=0 for external (receiving), change=1 for internal.
func (w *Wallet) DeriveAddress(account, change, index uint32) *ExtendedKey {
	return w.MasterKey.DeriveBIP44(account, change, index)
}

// ColdSign signs a message using the master (cold) key.
// Use this for high-value operations (large transfers, governance votes).
func (w *Wallet) ColdSign(message []byte) []byte {
	return ed25519.Sign(w.MasterKey.PrivateKey, message)
}

// HotSign signs a message using the hot (derived) key.
// Use this for everyday operations (small transfers, heartbeats).
func (w *Wallet) HotSign(message []byte) []byte {
	return ed25519.Sign(w.HotKey.PrivateKey, message)
}

// ── View-Only Wallet (Hot Wallet without private key) ──

// ViewOnlyWallet can verify signatures but cannot sign.
// Used by nodes that only need to verify a remote wallet's transactions.
type ViewOnlyWallet struct {
	PublicKey ed25519.PublicKey `json:"public_key"`
	NodeID    string            `json:"node_id"`
}

// NewViewOnlyWallet creates a view-only wallet from a public key hex string.
func NewViewOnlyWallet(publicKeyHex string) (*ViewOnlyWallet, error) {
	pubKey, err := hex.DecodeString(publicKeyHex)
	if err != nil || len(pubKey) != ed25519.PublicKeySize {
		return nil, fmt.Errorf("invalid public key")
	}
	return &ViewOnlyWallet{
		PublicKey: pubKey,
		NodeID:   deriveNodeID(pubKey),
	}, nil
}

// Verify checks a signature against this wallet's public key.
func (v *ViewOnlyWallet) Verify(message, signature []byte) bool {
	return ed25519.Verify(v.PublicKey, message, signature)
}

// ── Wallet File Persistence ──

type walletFile struct {
	PrivateKey []byte `json:"private_key"`
	PublicKey  []byte `json:"public_key"`
	Mnemonic   string `json:"mnemonic,omitempty"` // encrypted or blank for hot-only
	WalletType string `json:"wallet_type"`        // "full", "hot", "view_only"
}

// SaveWalletKey persists the wallet's master key to disk (compatible with existing .node_key format).
func SaveWalletKey(w *Wallet, keyFile string) error {
	stored := walletFile{
		PrivateKey: w.MasterKey.PrivateKey,
		PublicKey:  w.MasterKey.PublicKey,
		WalletType: "full",
	}
	data, err := json.Marshal(stored)
	if err != nil {
		return err
	}

	// Ensure directory exists
	if idx := strings.LastIndex(keyFile, "/"); idx >= 0 {
		os.MkdirAll(keyFile[:idx], 0700)
	}

	return os.WriteFile(keyFile, data, 0600)
}
