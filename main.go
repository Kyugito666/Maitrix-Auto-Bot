package main

import (
	"context"
	"crypto/ecdsa"
	"encoding/hex"
	"fmt"
	"log"
	"math/big"
	"os"
	"strings"
	"time"

	"github.com/ethereum/go-ethereum/accounts/abi"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/ethereum/go-ethereum/ethclient"
	"github.com/go-resty/resty/v2"
	"github.com/joho/godotenv"
	"github.com/manifoldco/promptui"
)

// ABI kontrak ERC20
const ERC20ABI = `[
	"function decimals() view returns (uint8)",
	"function balanceOf(address owner) view returns (uint256)",
	"function allowance(address owner, address spender) view returns (uint256)",
	"function approve(address spender, uint256 amount) external returns (bool)"
]`

// ABI untuk fungsi mint kustom
const ROUTER_ABI_MINT = `[
	{
		"inputs": [{"type": "uint256", "name": "amount"}],
		"name": "customMint",
		"outputs": [],
		"stateMutability": "nonpayable",
		"type": "function"
	}
]`

// ABI untuk staking
const STAKING_ABI = `[
	{
		"inputs": [{"type": "uint256", "name": "amount"}],
		"name": "stake",
		"outputs": [],
		"stateMutability": "nonpayable",
		"type": "function",
		"selector": "0xa694fc3a"
	},
	{
		"inputs": [],
		"name": "vault",
		"outputs": [{"type": "address"}],
		"stateMutability": "view",
		"type": "function"
	},
	{
		"inputs": [],
		"name": "ausd",
		"outputs": [{"type": "address"}],
		"stateMutability": "view",
		"type": "function"
	},
	{
		"inputs": [],
		"name": "token",
		"outputs": [{"type": "address"}],
		"stateMutability": "view",
		"type": "function"
	}
]`

// Struct untuk menyimpan konfigurasi dari file .env
type Config struct {
	PrivateKeys  []string
	RPC_URL      string
	ATH_ADDRESS  string
	AI16Z_ADDRESS string
	USDE_ADDRESS string
	VANA_ADDRESS string
	VIRTUAL_ADDRESS string
	LULUSD_ADDRESS string
	AZUSD_ADDRESS string
	VANAUSD_ADDRESS string
	AUSD_ADDRESS string
	VUSD_ADDRESS string
}

// Konfigurasi Token
var TOKEN_CONFIG = map[string]struct {
	RouterAddress   string
	Selector        string
	InputTokenAddress string
	OutputTokenAddress string
	InputTokenName  string
	MinAmount       float64
}{
	"AUSD": {
		RouterAddress:   "0x2cFDeE1d5f04dD235AEA47E1aD2fB66e3A61C13e",
		Selector:        "0x1bf6318b",
		InputTokenAddress: "0x1428444Eacdc0Fd115dd4318FcE65B61Cd1ef399",
		OutputTokenAddress: "0x78De28aABBD5198657B26A8dc9777f441551B477",
		InputTokenName:  "ATH",
		MinAmount:       50.0,
	},
	"VUSD": {
		RouterAddress:   "0x3dCACa90A714498624067948C092Dd0373f08265",
		Selector:        "0xa6d67510",
		InputTokenAddress: "0xFF27D611ab162d7827bbbA59F140C1E7aE56e95C",
		OutputTokenAddress: "0xc14A8E2Fc341A97a57524000bF0F7F1bA4de4802",
		InputTokenName:  "Virtual",
		MinAmount:       2.0,
	},
	"AZUSD": {
		RouterAddress:   "0xB0b53d8B4ef06F9Bbe5db624113C6A5D35bB7522",
		Selector:        "0xa6d67510",
		InputTokenAddress: "0x2d5a4f5634041f50180A25F26b2A8364452E3152",
		OutputTokenAddress: "0x5966cd11aED7D68705C9692e74e5688C892cb162",
		InputTokenName:  "Ai16Z",
		MinAmount:       5.0,
	},
	"VANAUSD": {
		RouterAddress:   "0xEfbAE3A68b17a61f21C7809Edfa8Aa3CA7B2546f",
		Selector:        "0xa6d67510",
		InputTokenAddress: "0xBEbF4E25652e7F23CCdCCcaaCB32004501c4BfF8",
		OutputTokenAddress: "0x46a6585a0Ad1750d37B4e6810EB59cBDf591Dc30",
		InputTokenName:  "VANA",
		MinAmount:       0.2,
	},
}

// Konfigurasi Staking
var STAKING_CONFIG = map[string]struct {
	StakingAddress        string
	TokenAddress          string
	TokenName             string
	MinAmount             float64
	RequiresTokenFunction bool
}{
	"AZUSD": {
		StakingAddress:        "0xf45Fde3F484C44CC35Bdc2A7fCA3DDDe0C8f252E",
		TokenAddress:          "0x5966cd11aED7D68705C9692e74e5688C892cb162",
		TokenName:             "azUSD",
		MinAmount:             0.0001,
		RequiresTokenFunction: true,
	},
	"AUSD": {
		StakingAddress:        "0x054de909723ECda2d119E31583D40a52a332f85c",
		TokenAddress:          "0x78De28aABBD5198657B26A8dc9777f441551B477",
		TokenName:             "AUSD",
		MinAmount:             0.0001,
		RequiresTokenFunction: false,
	},
	"VANAUSD": {
		StakingAddress:        "0x2608A88219BFB34519f635Dd9Ca2Ae971539ca60",
		TokenAddress:          "0x46a6585a0Ad1750d37B4e6810EB59cBDf591Dc30",
		TokenName:             "VANAUSD",
		MinAmount:             0.0001,
		RequiresTokenFunction: true,
	},
	"VUSD": {
		StakingAddress:        "0x5bb9Fa02a3DCCDB4E9099b48e8Ba5841D2e59d51",
		TokenAddress:          "0xc14A8E2Fc341A97a57524000bF0F7F1bA4de4802",
		TokenName:             "vUSD",
		MinAmount:             0.0001,
		RequiresTokenFunction: true,
	},
}

// Endpoint Faucet
var FAUCET_APIS = map[string]string{
	"ATH":      "https://app.x-network.io/maitrix-faucet/faucet",
	"USDe":     "https://app.x-network.io/maitrix-usde/faucet",
	"LULUSD":   "https://app.x-network.io/maitrix-lvl/faucet",
	"Ai16Z":    "https://app.x-network.io/maitrix-ai16z/faucet",
	"Virtual":  "https://app.x-network.io/maitrix-virtual/faucet",
	"Vana":     "https://app.x-network.io/maitrix-vana/faucet",
}

// Global variable untuk klien RPC
var ethClient *ethclient.Client
var cfg Config

func loadConfig() {
	err := godotenv.Load("accounts.env")
	if err != nil {
		log.Fatal("Error loading .env file")
	}

	keysStr := os.Getenv("PRIVATE_KEYS")
	if keysStr == "" {
		log.Fatal("PRIVATE_KEYS not set in .env file")
	}
	cfg.PrivateKeys = strings.Split(keysStr, ",")
	cfg.RPC_URL = os.Getenv("RPC_URL")
	cfg.ATH_ADDRESS = os.Getenv("ATH_ADDRESS")
	cfg.AI16Z_ADDRESS = os.Getenv("AI16Z_ADDRESS")
	cfg.USDE_ADDRESS = os.Getenv("USDE_ADDRESS")
	cfg.VANA_ADDRESS = os.Getenv("VANA_ADDRESS")
	cfg.VIRTUAL_ADDRESS = os.Getenv("VIRTUAL_ADDRESS")
	cfg.LULUSD_ADDRESS = os.Getenv("LULUSD_ADDRESS")
	cfg.AZUSD_ADDRESS = os.Getenv("AZUSD_ADDRESS")
	cfg.VANAUSD_ADDRESS = os.Getenv("VANAUSD_ADDRESS")
	cfg.AUSD_ADDRESS = os.Getenv("AUSD_ADDRESS")
	cfg.VUSD_ADDRESS = os.Getenv("VUSD_ADDRESS")
}

func getPrivateKey(accountIndex int) (*ecdsa.PrivateKey, error) {
	if accountIndex >= len(cfg.PrivateKeys) {
		return nil, fmt.Errorf("account index %d out of bounds", accountIndex)
	}
	privateKey, err := crypto.HexToECDSA(cfg.PrivateKeys[accountIndex])
	if err != nil {
		return nil, fmt.Errorf("invalid private key: %v", err)
	}
	return privateKey, nil
}

func getWalletAddress(privateKey *ecdsa.PrivateKey) common.Address {
	publicKey := privateKey.Public()
	publicKeyECDSA, ok := publicKey.(*ecdsa.PublicKey)
	if !ok {
		log.Fatal("cannot assert type: publicKey is not of type *ecdsa.PublicKey")
	}
	return crypto.PubkeyToAddress(*publicKeyECDSA)
}

func getTokenBalance(tokenAddress common.Address, ownerAddress common.Address) (*big.Int, uint8, error) {
	tokenABI, err := abi.JSON(strings.NewReader(ERC20ABI))
	if err != nil {
		return nil, 0, err
	}
	tokenContract := common.NewBoundContract(tokenAddress, tokenABI, ethClient, ethClient, nil)

	var balance *big.Int
	var decimals uint8
	err = tokenContract.Call(nil, &balance, "balanceOf", ownerAddress)
	if err != nil {
		return nil, 0, err
	}
	err = tokenContract.Call(nil, &decimals, "decimals")
	if err != nil {
		return nil, 0, err
	}
	return balance, decimals, nil
}

func updateWalletData(privateKey *ecdsa.PrivateKey) {
	address := getWalletAddress(privateKey)
	fmt.Printf("Updating wallet data for address: %s\n", address.Hex())

	balance, err := ethClient.BalanceAt(context.Background(), address, nil)
	if err != nil {
		log.Printf("Failed to get ETH balance: %v", err)
	} else {
		ethBalance := new(big.Float).SetInt(balance)
		ethValue := new(big.Float).Quo(ethBalance, big.NewFloat(1e18))
		fmt.Printf("ETH Balance: %s\n", ethValue.String())
	}

	tokenAddresses := map[string]common.Address{
		"ATH":      common.HexToAddress(cfg.ATH_ADDRESS),
		"AI16Z":    common.HexToAddress(cfg.AI16Z_ADDRESS),
		"USDE":     common.HexToAddress(cfg.USDE_ADDRESS),
		"VANA":     common.HexToAddress(cfg.VANA_ADDRESS),
		"VIRTUAL":  common.HexToAddress(cfg.VIRTUAL_ADDRESS),
		"LULUSD":   common.HexToAddress(cfg.LULUSD_ADDRESS),
		"AZUSD":    common.HexToAddress(cfg.AZUSD_ADDRESS),
		"VANAUSD":  common.HexToAddress(cfg.VANAUSD_ADDRESS),
		"AUSD":     common.HexToAddress(cfg.AUSD_ADDRESS),
		"VUSD":     common.HexToAddress(cfg.VUSD_ADDRESS),
	}

	for name, addr := range tokenAddresses {
		balance, decimals, err := getTokenBalance(addr, address)
		if err != nil {
			log.Printf("Failed to get %s balance: %v", name, err)
			continue
		}
		floatBalance := new(big.Float).SetInt(balance)
		divisor := new(big.Float).SetInt(new(big.Int).Exp(big.NewInt(10), big.NewInt(int64(decimals)), nil))
		tokenValue := new(big.Float).Quo(floatBalance, divisor)
		fmt.Printf("%s Balance: %s\n", name, tokenValue.String())
	}
}

func claimFaucet(privateKey *ecdsa.PrivateKey, token string) {
	fmt.Printf("Claiming %s for %s\n", token, getWalletAddress(privateKey).Hex())
	apiUrl, ok := FAUCET_APIS[token]
	if !ok {
		log.Printf("API for token %s not found", token)
		return
	}

	restyClient := resty.New()
	payload := map[string]string{"address": getWalletAddress(privateKey).Hex()}
	resp, err := restyClient.R().
		SetHeader("Content-Type", "application/json").
		SetBody(payload).
		Post(apiUrl)

	if err != nil {
		log.Printf("Error claiming %s: %v", token, err)
		return
	}

	var result struct {
		Code    int    `json:"code"`
		Message string `json:"message"`
	}
	err = resp.UnmarshalJson(&result)
	if err != nil {
		log.Printf("Error parsing response for %s: %v", token, err)
		return
	}

	if result.Code == 200 {
		fmt.Printf("Successfully claimed %s!\n", token)
	} else {
		fmt.Printf("Failed to claim %s: %s\n", token, result.Message)
	}
}

func mintToken(privateKey *ecdsa.PrivateKey, token string, amount float64) {
	fmt.Printf("Minting %s for %s\n", token, getWalletAddress(privateKey).Hex())

	config, ok := TOKEN_CONFIG[token]
	if !ok {
		log.Printf("Token %s not supported for minting.", token)
		return
	}

	routerAddress := common.HexToAddress(config.RouterAddress)
	inputTokenAddress := common.HexToAddress(config.InputTokenAddress)
	inputTokenName := config.InputTokenName
	selector, _ := hex.DecodeString(config.Selector[2:])

	parsedABI, err := abi.JSON(strings.NewReader(ERC20ABI))
	if err != nil {
		log.Printf("Failed to parse ERC20 ABI: %v", err)
		return
	}

	inputTokenContract := common.NewBoundContract(inputTokenAddress, parsedABI, ethClient, ethClient, nil)
	var decimals uint8
	err = inputTokenContract.Call(nil, &decimals, "decimals")
	if err != nil {
		log.Printf("Failed to get decimals for %s: %v", inputTokenName, err)
		return
	}

	amountWei := toWei(amount, int64(decimals))
	allowance, _, err := inputTokenContract.Call(nil, nil, "allowance", getWalletAddress(privateKey), routerAddress)
	if err != nil {
		log.Printf("Failed to get allowance: %v", err)
		return
	}

	if allowance.(*big.Int).Cmp(amountWei) < 0 {
		fmt.Printf("Approving %f %s...\n", amount, inputTokenName)
		tx, err := inputTokenContract.Transact(nil, "approve", routerAddress, amountWei)
		if err != nil {
			log.Printf("Approval failed: %v", err)
			return
		}
		receipt, err := bindWait(tx)
		if err != nil || receipt.Status != types.ReceiptStatusSuccessful {
			log.Printf("Approval transaction failed or reverted: %v", err)
			return
		}
		fmt.Printf("Approval successful. TxHash: %s\n", receipt.TxHash.Hex())
	}

	routerABI, err := abi.JSON(strings.NewReader(ROUTER_ABI_MINT))
	if err != nil {
		log.Printf("Failed to parse router ABI: %v", err)
		return
	}

	data, err := routerABI.Pack("customMint", amountWei)
	if err != nil {
		log.Printf("Failed to pack data: %v", err)
		return
	}

	// This part is a bit tricky to replicate in Go's ethclient with a simple "sendTransaction" with custom data.
	// We'll use the low-level method to create and sign the transaction.
	nonce, err := ethClient.PendingNonceAt(context.Background(), getWalletAddress(privateKey))
	if err != nil {
		log.Printf("Failed to get nonce: %v", err)
		return
	}

	gasPrice, err := ethClient.SuggestGasPrice(context.Background())
	if err != nil {
		log.Printf("Failed to get gas price: %v", err)
		return
	}

	tx := types.NewTransaction(nonce, routerAddress, big.NewInt(0), 250000, gasPrice, data)
	chainID, err := ethClient.NetworkID(context.Background())
	if err != nil {
		log.Printf("Failed to get chain ID: %v", err)
		return
	}

	signedTx, err := types.SignTx(tx, types.NewEIP155Signer(chainID), privateKey)
	if err != nil {
		log.Printf("Failed to sign transaction: %v", err)
		return
	}

	err = ethClient.SendTransaction(context.Background(), signedTx)
	if err != nil {
		log.Printf("Failed to send transaction: %v", err)
		return
	}

	fmt.Printf("Mint transaction sent. TxHash: %s\n", signedTx.Hash().Hex())
	receipt, err := bindWait(signedTx)
	if err != nil || receipt.Status != types.ReceiptStatusSuccessful {
		log.Printf("Mint transaction failed or reverted: %v", err)
	} else {
		fmt.Printf("Mint successful! TxHash: %s\n", receipt.TxHash.Hex())
	}
}

func stakeToken(privateKey *ecdsa.PrivateKey, token string, amount float64) {
	fmt.Printf("Staking %s for %s\n", token, getWalletAddress(privateKey).Hex())

	config, ok := STAKING_CONFIG[token]
	if !ok {
		log.Printf("Token %s not supported for staking.", token)
		return
	}
	stakingAddress := common.HexToAddress(config.StakingAddress)
	tokenAddress := common.HexToAddress(config.TokenAddress)
	tokenName := config.TokenName

	tokenABI, err := abi.JSON(strings.NewReader(ERC20ABI))
	if err != nil {
		log.Printf("Failed to parse ERC20 ABI: %v", err)
		return
	}
	tokenContract := common.NewBoundContract(tokenAddress, tokenABI, ethClient, ethClient, nil)

	var decimals uint8
	err = tokenContract.Call(nil, &decimals, "decimals")
	if err != nil {
		log.Printf("Failed to get decimals for %s: %v", tokenName, err)
		return
	}

	amountWei := toWei(amount, int64(decimals))
	allowance, _, err := tokenContract.Call(nil, nil, "allowance", getWalletAddress(privateKey), stakingAddress)
	if err != nil {
		log.Printf("Failed to get allowance: %v", err)
		return
	}

	if allowance.(*big.Int).Cmp(amountWei) < 0 {
		fmt.Printf("Approving %f %s...\n", amount, tokenName)
		tx, err := tokenContract.Transact(nil, "approve", stakingAddress, amountWei)
		if err != nil {
			log.Printf("Approval failed: %v", err)
			return
		}
		receipt, err := bindWait(tx)
		if err != nil || receipt.Status != types.ReceiptStatusSuccessful {
			log.Printf("Approval transaction failed or reverted: %v", err)
			return
		}
		fmt.Printf("Approval successful. TxHash: %s\n", receipt.TxHash.Hex())
	}

	stakingABI, err := abi.JSON(strings.NewReader(STAKING_ABI))
	if err != nil {
		log.Printf("Failed to parse staking ABI: %v", err)
		return
	}
	data, err := stakingABI.Pack("stake", amountWei)
	if err != nil {
		log.Printf("Failed to pack data: %v", err)
		return
	}
	
	nonce, err := ethClient.PendingNonceAt(context.Background(), getWalletAddress(privateKey))
	if err != nil {
		log.Printf("Failed to get nonce: %v", err)
		return
	}
	gasPrice, err := ethClient.SuggestGasPrice(context.Background())
	if err != nil {
		log.Printf("Failed to get gas price: %v", err)
		return
	}

	tx := types.NewTransaction(nonce, stakingAddress, big.NewInt(0), 300000, gasPrice, data)
	chainID, err := ethClient.NetworkID(context.Background())
	if err != nil {
		log.Printf("Failed to get chain ID: %v", err)
		return
	}

	signedTx, err := types.SignTx(tx, types.NewEIP155Signer(chainID), privateKey)
	if err != nil {
		log.Printf("Failed to sign transaction: %v", err)
		return
	}

	err = ethClient.SendTransaction(context.Background(), signedTx)
	if err != nil {
		log.Printf("Failed to send transaction: %v", err)
		return
	}
	fmt.Printf("Staking transaction sent. TxHash: %s\n", signedTx.Hash().Hex())

	receipt, err := bindWait(signedTx)
	if err != nil || receipt.Status != types.ReceiptStatusSuccessful {
		log.Printf("Staking transaction failed or reverted: %v", err)
	} else {
		fmt.Printf("Staking successful! TxHash: %s\n", receipt.TxHash.Hex())
	}
}

func toWei(amount float64, decimals int64) *big.Int {
	floatAmount := big.NewFloat(amount)
	wei := big.NewFloat(1)
	wei.SetInt(new(big.Int).Exp(big.NewInt(10), big.NewInt(decimals), nil))
	result := new(big.Float).Mul(floatAmount, wei)
	intResult, _ := result.Int(nil)
	return intResult
}

func bindWait(tx *types.Transaction) (*types.Receipt, error) {
	fmt.Printf("Waiting for transaction %s to be confirmed...\n", tx.Hash().Hex())
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	return ethClient.TransactionReceipt(ctx, tx.Hash())
}

func runAutoClaimAllFaucet(privateKey *ecdsa.PrivateKey) {
	fmt.Println("Starting Auto Claim All Faucet...")
	tokens := []string{"ATH", "USDe", "LULUSD", "Ai16Z", "Virtual", "Vana"}
	for _, token := range tokens {
		claimFaucet(privateKey, token)
		time.Sleep(5 * time.Second)
	}
	fmt.Println("Auto Claim All Faucet completed.")
}

func runAutoMint(privateKey *ecdsa.PrivateKey) {
	prompt := promptui.Select{
		Label: "Select a token to mint",
		Items: []string{"AUSD", "VUSD", "AZUSD", "VANAUSD", "Back"},
	}
	_, result, err := prompt.Run()
	if err != nil {
		fmt.Printf("Prompt failed %v\n", err)
		return
	}

	if result == "Back" {
		return
	}

	config := TOKEN_CONFIG[result]
	promptAmount := promptui.Prompt{
		Label:   fmt.Sprintf("Enter amount to mint (min %f %s)", config.MinAmount, config.InputTokenName),
		Default: fmt.Sprintf("%f", config.MinAmount),
	}
	amountStr, err := promptAmount.Run()
	if err != nil {
		fmt.Printf("Prompt failed %v\n", err)
		return
	}

	var amount float64
	fmt.Sscanf(amountStr, "%f", &amount)
	if amount < config.MinAmount {
		fmt.Printf("Minimum amount is %f\n", config.MinAmount)
		return
	}
	mintToken(privateKey, result, amount)
}

func runAutoStake(privateKey *ecdsa.PrivateKey) {
	prompt := promptui.Select{
		Label: "Select a token to stake",
		Items: []string{"AZUSD", "AUSD", "VANAUSD", "VUSD", "Back"},
	}
	_, result, err := prompt.Run()
	if err != nil {
		fmt.Printf("Prompt failed %v\n", err)
		return
	}

	if result == "Back" {
		return
	}

	config := STAKING_CONFIG[result]
	promptAmount := promptui.Prompt{
		Label:   fmt.Sprintf("Enter amount to stake (min %f %s)", config.MinAmount, config.TokenName),
		Default: fmt.Sprintf("%f", config.MinAmount),
	}
	amountStr, err := promptAmount.Run()
	if err != nil {
		fmt.Printf("Prompt failed %v\n", err)
		return
	}

	var amount float64
	fmt.Sscanf(amountStr, "%f", &amount)
	if amount < config.MinAmount {
		fmt.Printf("Minimum amount is %f\n", config.MinAmount)
		return
	}
	stakeToken(privateKey, result, amount)
}

func main() {
	loadConfig()
	client, err := ethclient.Dial(cfg.RPC_URL)
	if err != nil {
		log.Fatalf("Failed to connect to the Ethereum client: %v", err)
	}
	ethClient = client

	for i, key := range cfg.PrivateKeys {
		privateKey, err := getPrivateKey(i)
		if err != nil {
			log.Printf("Skipping account %d due to invalid private key: %v", i+1, err)
			continue
		}
		fmt.Printf("\n==================== Starting Operations for Account %d ====================\n", i+1)
		updateWalletData(privateKey)

		// Loop utama untuk setiap akun
		for {
			prompt := promptui.Select{
				Label: "Select action for " + getWalletAddress(privateKey).Hex(),
				Items: []string{"Auto Claim All Faucet", "Auto Mint Token", "Auto Stake", "Next Account", "Exit"},
			}
			_, result, err := prompt.Run()
			if err != nil {
				fmt.Printf("Prompt failed %v\n", err)
				return
			}

			switch result {
			case "Auto Claim All Faucet":
				runAutoClaimAllFaucet(privateKey)
			case "Auto Mint Token":
				runAutoMint(privateKey)
			case "Auto Stake":
				runAutoStake(privateKey)
			case "Next Account":
				goto nextAccount
			case "Exit":
				return
			}
		}
	nextAccount:
		fmt.Println("Moving to the next account...")
	}
}
