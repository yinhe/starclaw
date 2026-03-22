package handler

import (
	"bytes"
	"crypto"
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"crypto/rsa"
	"crypto/sha256"
	"crypto/x509"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"encoding/pem"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"time"

	"github.com/yinhe/starclaw-router/internal/config"
)

// WechatPayClient handles WeChat Pay V3 API calls.
type WechatPayClient struct {
	cfg        config.WechatConfig
	privateKey *rsa.PrivateKey
	publicKey  *rsa.PublicKey // WeChat platform public key (for callback verification)
}

// NewWechatPayClient initializes the WeChat Pay V3 client.
func NewWechatPayClient(cfg config.WechatConfig) *WechatPayClient {
	c := &WechatPayClient{cfg: cfg}

	// Load merchant private key
	keyPEM, err := os.ReadFile(cfg.PrivateKeyPath)
	if err != nil {
		log.Printf("[star-ai] WeChat Pay private key not found: %v", err)
		return nil
	}
	block, _ := pem.Decode(keyPEM)
	if block == nil {
		log.Printf("[star-ai] WeChat Pay private key PEM decode failed")
		return nil
	}
	key, err := x509.ParsePKCS8PrivateKey(block.Bytes)
	if err != nil {
		// Try PKCS1 format as fallback
		key2, err2 := x509.ParsePKCS1PrivateKey(block.Bytes)
		if err2 != nil {
			log.Printf("[star-ai] WeChat Pay private key parse error: %v / %v", err, err2)
			return nil
		}
		c.privateKey = key2
	} else {
		rsaKey, ok := key.(*rsa.PrivateKey)
		if !ok {
			log.Printf("[star-ai] WeChat Pay private key is not RSA")
			return nil
		}
		c.privateKey = rsaKey
	}

	// Load WeChat platform public key (for verifying callbacks)
	if cfg.PublicKeyPath != "" {
		pubPEM, err := os.ReadFile(cfg.PublicKeyPath)
		if err != nil {
			log.Printf("[star-ai] WeChat Pay public key not found: %v", err)
		} else {
			block, _ := pem.Decode(pubPEM)
			if block != nil {
				pubKey, err := x509.ParsePKIXPublicKey(block.Bytes)
				if err != nil {
					// Try parsing as certificate
					cert, err2 := x509.ParseCertificate(block.Bytes)
					if err2 != nil {
						log.Printf("[star-ai] WeChat Pay public key parse error: %v / %v", err, err2)
					} else {
						if rsaPub, ok := cert.PublicKey.(*rsa.PublicKey); ok {
							c.publicKey = rsaPub
						}
					}
				} else {
					if rsaPub, ok := pubKey.(*rsa.PublicKey); ok {
						c.publicKey = rsaPub
					}
				}
			}
		}
	}

	log.Printf("[star-ai] WeChat Pay client initialized (appID=%s, mchID=%s)", cfg.AppID, cfg.MchID)
	return c
}

// ── Native Order Creation ──

// NativeOrderRequest is the request body for WeChat Pay V3 Native order.
type NativeOrderRequest struct {
	AppID       string            `json:"appid"`
	MchID       string            `json:"mchid"`
	Description string            `json:"description"`
	OutTradeNo  string            `json:"out_trade_no"`
	NotifyURL   string            `json:"notify_url"`
	Amount      NativeOrderAmount `json:"amount"`
}

type NativeOrderAmount struct {
	Total    int    `json:"total"` // amount in fen (分)
	Currency string `json:"currency"`
}

// NativeOrderResponse is the response from WeChat Pay V3.
type NativeOrderResponse struct {
	CodeURL string `json:"code_url"` // QR code content URL
}

// CreateNativeOrder creates a WeChat Pay Native order and returns the QR code URL.
func (c *WechatPayClient) CreateNativeOrder(description, outTradeNo string, amountFen int) (string, error) {
	reqBody := NativeOrderRequest{
		AppID:       c.cfg.AppID,
		MchID:       c.cfg.MchID,
		Description: description,
		OutTradeNo:  outTradeNo,
		NotifyURL:   c.cfg.NotifyURL,
		Amount: NativeOrderAmount{
			Total:    amountFen,
			Currency: "CNY",
		},
	}

	bodyBytes, err := json.Marshal(reqBody)
	if err != nil {
		return "", fmt.Errorf("marshal request: %w", err)
	}

	apiURL := "https://api.mch.weixin.qq.com/v3/pay/transactions/native"
	resp, err := c.doSignedRequest("POST", apiURL, bodyBytes)
	if err != nil {
		return "", fmt.Errorf("API request: %w", err)
	}
	defer resp.Body.Close()

	respBody, _ := io.ReadAll(resp.Body)

	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusNoContent {
		return "", fmt.Errorf("WeChat API %d: %s", resp.StatusCode, string(respBody))
	}

	var result NativeOrderResponse
	if err := json.Unmarshal(respBody, &result); err != nil {
		return "", fmt.Errorf("unmarshal response: %w (body: %s)", err, string(respBody))
	}

	if result.CodeURL == "" {
		return "", fmt.Errorf("empty code_url in response: %s", string(respBody))
	}

	return result.CodeURL, nil
}

// ── Order Query ──

// QueryOrder queries WeChat Pay V3 for order status by out_trade_no.
func (c *WechatPayClient) QueryOrder(outTradeNo string) (*WechatPayResult, error) {
	apiURL := fmt.Sprintf("https://api.mch.weixin.qq.com/v3/pay/transactions/out-trade-no/%s?mchid=%s",
		outTradeNo, c.cfg.MchID)

	resp, err := c.doSignedRequest("GET", apiURL, nil)
	if err != nil {
		return nil, fmt.Errorf("API request: %w", err)
	}
	defer resp.Body.Close()

	respBody, _ := io.ReadAll(resp.Body)

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("WeChat API %d: %s", resp.StatusCode, string(respBody))
	}

	var result WechatPayResult
	if err := json.Unmarshal(respBody, &result); err != nil {
		return nil, fmt.Errorf("unmarshal response: %w (body: %s)", err, string(respBody))
	}

	return &result, nil
}

// ── Callback Verification ──

// WechatNotification is the outer envelope of a WeChat Pay V3 callback.
type WechatNotification struct {
	ID           string                     `json:"id"`
	CreateTime   string                     `json:"create_time"`
	EventType    string                     `json:"event_type"`
	ResourceType string                     `json:"resource_type"`
	Resource     WechatNotificationResource `json:"resource"`
	Summary      string                     `json:"summary"`
}

type WechatNotificationResource struct {
	Algorithm      string `json:"algorithm"`
	Ciphertext     string `json:"ciphertext"`
	AssociatedData string `json:"associated_data"`
	Nonce          string `json:"nonce"`
	OriginalType   string `json:"original_type"`
}

// WechatPayResult is the decrypted payment result.
type WechatPayResult struct {
	AppID          string `json:"appid"`
	MchID          string `json:"mchid"`
	OutTradeNo     string `json:"out_trade_no"`
	TransactionID  string `json:"transaction_id"`
	TradeType      string `json:"trade_type"`
	TradeState     string `json:"trade_state"`
	TradeStateDesc string `json:"trade_state_desc"`
	Amount         struct {
		Total         int    `json:"total"`
		PayerTotal    int    `json:"payer_total"`
		Currency      string `json:"currency"`
		PayerCurrency string `json:"payer_currency"`
	} `json:"amount"`
	Payer struct {
		OpenID string `json:"openid"`
	} `json:"payer"`
	SuccessTime string `json:"success_time"`
}

// VerifyAndDecryptCallback verifies the callback signature and decrypts the resource.
func (c *WechatPayClient) VerifyAndDecryptCallback(
	timestamp, nonce, body, signature string,
) (*WechatPayResult, error) {
	// 1. Verify signature (if public key available)
	if c.publicKey != nil {
		signStr := fmt.Sprintf("%s\n%s\n%s\n", timestamp, nonce, body)
		sigBytes, err := base64.StdEncoding.DecodeString(signature)
		if err != nil {
			return nil, fmt.Errorf("decode signature: %w", err)
		}
		h := sha256.Sum256([]byte(signStr))
		if err := rsa.VerifyPKCS1v15(c.publicKey, crypto.SHA256, h[:], sigBytes); err != nil {
			return nil, fmt.Errorf("signature verification failed: %w", err)
		}
	} else {
		log.Printf("[star-ai] WeChat Pay: no public key loaded, skipping signature verification")
	}

	// 2. Parse notification envelope
	var noti WechatNotification
	if err := json.Unmarshal([]byte(body), &noti); err != nil {
		return nil, fmt.Errorf("parse notification: %w", err)
	}

	// 3. Decrypt resource with AES-256-GCM using APIv3 key
	plaintext, err := decryptAESGCM(
		c.cfg.APIv3Key,
		noti.Resource.Nonce,
		noti.Resource.Ciphertext,
		noti.Resource.AssociatedData,
	)
	if err != nil {
		return nil, fmt.Errorf("decrypt resource: %w", err)
	}

	// 4. Parse payment result
	var result WechatPayResult
	if err := json.Unmarshal(plaintext, &result); err != nil {
		return nil, fmt.Errorf("parse payment result: %w", err)
	}

	return &result, nil
}

// ── V3 Request Signing ──

func (c *WechatPayClient) doSignedRequest(method, url string, body []byte) (*http.Response, error) {
	ts := fmt.Sprintf("%d", time.Now().Unix())
	nonce := generateNonce()

	// URL path extraction
	urlPath := url
	if idx := len("https://api.mch.weixin.qq.com"); idx < len(url) {
		urlPath = url[idx:]
	}

	// Build sign string
	signStr := fmt.Sprintf("%s\n%s\n%s\n%s\n%s\n", method, urlPath, ts, nonce, string(body))
	sig, err := c.sign(signStr)
	if err != nil {
		return nil, fmt.Errorf("sign request: %w", err)
	}

	// Determine serial_no: use PublicKeyID if available (new API), else MchSerialNo
	serialNo := c.cfg.MchSerialNo
	authType := "WECHATPAY2-SHA256-RSA2048"
	if c.cfg.PublicKeyID != "" {
		authType = "WECHATPAY2-SHA256-RSA2048"
	}

	authHeader := fmt.Sprintf(
		`%s mchid="%s",nonce_str="%s",timestamp="%s",serial_no="%s",signature="%s"`,
		authType, c.cfg.MchID, nonce, ts, serialNo, sig,
	)

	req, err := http.NewRequest(method, url, bytes.NewReader(body))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")
	req.Header.Set("Authorization", authHeader)
	req.Header.Set("User-Agent", "StarAI-Server/1.0")
	if c.cfg.PublicKeyID != "" {
		req.Header.Set("Wechatpay-Serial", c.cfg.PublicKeyID)
	}

	return http.DefaultClient.Do(req)
}

func (c *WechatPayClient) sign(message string) (string, error) {
	h := sha256.Sum256([]byte(message))
	sig, err := rsa.SignPKCS1v15(rand.Reader, c.privateKey, crypto.SHA256, h[:])
	if err != nil {
		return "", err
	}
	return base64.StdEncoding.EncodeToString(sig), nil
}

// ── AES-256-GCM Decryption (for callback resource) ──

func decryptAESGCM(apiV3Key, nonceStr, ciphertext, associatedData string) ([]byte, error) {
	// APIv3 key is 32 bytes (used directly as AES-256 key)
	key := []byte(apiV3Key)
	if len(key) != 32 {
		return nil, fmt.Errorf("APIv3 key must be 32 bytes, got %d", len(key))
	}

	ciphertextBytes, err := base64.StdEncoding.DecodeString(ciphertext)
	if err != nil {
		return nil, fmt.Errorf("decode ciphertext: %w", err)
	}

	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, err
	}

	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, err
	}

	plaintext, err := gcm.Open(nil, []byte(nonceStr), ciphertextBytes, []byte(associatedData))
	if err != nil {
		return nil, fmt.Errorf("GCM decrypt: %w", err)
	}

	return plaintext, nil
}

func generateNonce() string {
	b := make([]byte, 16)
	rand.Read(b)
	return hex.EncodeToString(b)
}
