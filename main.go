package main

import (
	"bytes"
	"context"
	"crypto/ecdsa"
	"crypto/rand"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"math/big"
	"mime/multipart"
	"net/http"
	"os"
	"skygate/abi"
	"strconv"

	"github.com/ethereum/go-ethereum/accounts/abi/bind"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/common/hexutil"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/ethereum/go-ethereum/ethclient"
	"github.com/joho/godotenv"
)

var (
	privateKey *ecdsa.PrivateKey
	eclient    *ethclient.Client
	skygate    *abi.Abi
	plusNumber int
	maxTimes   int
)

func init() {
	// Get environment pk
	err := godotenv.Load()
	if err != nil {
		panic(err)
	}
	privateKeyStr := os.Getenv("PK")
	privateKey, err = crypto.HexToECDSA(privateKeyStr)
	if err != nil {
		panic(err)
	}
	eclient, err = ethclient.Dial("https://opbnb.publicnode.com")
	if err != nil {
		panic(err)
	}
	address := common.HexToAddress("0x9465fe0e8cdf4e425e0c59b7caeccc1777dc6695")
	skygate, err = abi.NewAbi(address, eclient)
	if err != nil {
		log.Fatalf("Failed to instantiate a Token contract: %v", err)
	}

	plusNumberStr := os.Getenv("PLUSNUMBER")
	plusNumber, err = strconv.Atoi(plusNumberStr)
	if err != nil {
		log.Fatal("Wrong plus number")
	}
	maxTimesStr := os.Getenv("MAXTIMES")
	maxTimes, err = strconv.Atoi(maxTimesStr)
	if err != nil {
		log.Fatal("Wrong max times")
	}

}

func signMessage(message string, privateKey *ecdsa.PrivateKey) (string, error) {
	// Prepare the message
	message = fmt.Sprintf("\x19Ethereum Signed Message:\n%d%s", len(message), message)
	data := []byte(message)
	hash := crypto.Keccak256Hash(data)
	signature, err := crypto.Sign(hash.Bytes(), privateKey)
	if err != nil {
		return "", err
	}

	// Set v to 27
	var v byte = 27
	signature[64] = v
	signatureStr := hexutil.Encode(signature)
	return signatureStr, nil
}

func generateBoundary() string {
	dict := []byte("abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789")
	bytes := make([]byte, 16)
	rand.Read(bytes)
	for i, b := range bytes {
		bytes[i] = dict[b%byte(len(dict))]
	}
	return "----WebKitFormBoundary" + string(bytes)
}

type Fields struct {
	Buffer   *bytes.Buffer
	Boundary string
}

func generateFields(fields map[string]string) (Fields, error) {
	// Create form
	buffer := &bytes.Buffer{}
	writer := multipart.NewWriter(buffer)

	// Set Boundary
	boundary := generateBoundary()
	writer.SetBoundary(boundary)

	// Form Fields

	for key, val := range fields {
		fieldWriter, err := writer.CreateFormField(key)
		if err != nil {
			return Fields{}, err
		}
		_, err = fieldWriter.Write([]byte(val))
		if err != nil {
			return Fields{}, err
		}
	}
	if err := writer.Close(); err != nil {
		return Fields{}, err
	}
	return Fields{Buffer: buffer, Boundary: boundary}, nil
}

func login() (string, error) {
	url := "https://apisky.ntoken.bwtechnology.net/api/wallet_signin.php"

	signature, err := signMessage("skygate", privateKey)
	if err != nil {
		return "", err
	}

	fields := map[string]string{
		"api_id":      "skyark_react_api",
		"api_token":   "3C2D36F79AFB3D5374A49BE767A17C6A3AEF91635BF7A3FB25CEA8D4DD",
		"uWalletAddr": crypto.PubkeyToAddress(privateKey.PublicKey).Hex(),
		"sign":        signature,
	}

	fieldsWithBoundary, err := generateFields(fields)
	if err != nil {
		return "", err
	}

	req, err := http.NewRequest("POST", url, fieldsWithBoundary.Buffer)
	if err != nil {
		return "", err
	}
	req.Header = http.Header{
		"Content-Type": []string{fmt.Sprintf("multipart/form-data; boundary=%s", fieldsWithBoundary.Boundary)},
		"User-Agent":   []string{"Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/120.0.0.0 Safari/537.36"},
		"Origin":       []string{"https://skygate.skyarkchronicles.com"},
		"Referer":      []string{"https://skygate.skyarkchronicles.com"},
	}
	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	if resp.StatusCode != 200 {
		return "", errors.New("Log in Failed.")
	}
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", err
	}
	var result map[string]interface{}
	json.Unmarshal(body, &result)
	jwt, jwtOk := result["jwt"].(string)
	if !jwtOk {
		return "", errors.New("Can't parse response")
	}
	log.Println("Log In Success.")
	return jwt, nil
}

func signIn() (string, error) {
	auth, err := bind.NewKeyedTransactorWithChainID(privateKey, big.NewInt(204))
	if err != nil {
		return "", err
	}

	auth.Value = big.NewInt(0)
	auth.GasLimit = uint64(300000)

	tx, err := skygate.Signin(auth, big.NewInt(1))
	if err != nil {
		return "", err
	}
	receipt, err := bind.WaitMined(context.Background(), eclient, tx)
	if err != nil {
		return "", err
	}
	if receipt.Status == 1 {
		return tx.Hash().Hex(), nil
	} else {
		return "", errors.New("Tx not mined.")
	}
}

func checkIn(jwt string) error {
	url := "https://apisky.ntoken.bwtechnology.net/api/checkIn_skyGate_member.php"
	fields := map[string]string{
		"api_id":    "skyark_react_api",
		"api_token": "3C2D36F79AFB3D5374A49BE767A17C6A3AEF91635BF7A3FB25CEA8D4DD",
		"jwt":       jwt,
	}
	fieldsWithBoundary, err := generateFields(fields)
	if err != nil {
		return err
	}
	req, err := http.NewRequest("POST", url, fieldsWithBoundary.Buffer)
	if err != nil {
		return err
	}
	req.Header = http.Header{
		"Content-Type": []string{fmt.Sprintf("multipart/form-data; boundary=%s", fieldsWithBoundary.Boundary)},
		"User-Agent":   []string{"Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/120.0.0.0 Safari/537.36"},
		"Origin":       []string{"https://skygate.skyarkchronicles.com"},
		"Referer":      []string{"https://skygate.skyarkchronicles.com"},
	}
	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	log.Println("Check In Status", resp.Status)
	if resp.StatusCode != 200 {
		return errors.New("Check In Failed.")
	}
	log.Fatal("Check in Success.")
	return nil
}

func getCoinsNumber(jwt string) (int, error) {
	url := "https://apisky.ntoken.bwtechnology.net/api/get_skyGate_coin.php"
	fields := map[string]string{
		"api_id":    "skyark_react_api",
		"api_token": "3C2D36F79AFB3D5374A49BE767A17C6A3AEF91635BF7A3FB25CEA8D4DD",
		"jwt":       jwt,
	}
	fieldsWithBoundary, err := generateFields(fields)
	if err != nil {
		return 0, err
	}

	req, err := http.NewRequest("POST", url, fieldsWithBoundary.Buffer)
	if err != nil {
		return 0, err
	}
	req.Header = http.Header{
		"Content-Type": []string{fmt.Sprintf("multipart/form-data; boundary=%s", fieldsWithBoundary.Boundary)},
		"User-Agent":   []string{"Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/120.0.0.0 Safari/537.36"},
		"Origin":       []string{"https://skygate.skyarkchronicles.com"},
		"Referer":      []string{"https://skygate.skyarkchronicles.com"},
	}
	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		return 0, err
	}
	defer resp.Body.Close()
	log.Println("Get Coin Number Status ", resp.Status)
	if resp.StatusCode != 200 {
		return 0, errors.New("Get Coin Number Failed")
	}
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return 0, err
	}
	var result map[string]interface{}
	json.Unmarshal(body, &result)
	coinNumber, coinNumberOk := result["coin"].(string)
	if !coinNumberOk {
		return 0, errors.New("Can't parse response")
	}
	returnNumber, err := strconv.Atoi(coinNumber)
	if err != nil {
		return 0, err
	}
	return returnNumber, nil
}

func dailyCheckIn(jwt string, targetNumber int) error {
	for checkInTimes := 0; checkInTimes < maxTimes; {
		currentNumber, err := getCoinsNumber(jwt)
		if err != nil {
			continue
		}
		log.Println("当前积分: ", currentNumber)
		if currentNumber >= targetNumber {
			break
		}
		txhash, err := signIn()
		if err != nil {
			continue
		}
		log.Println("成功提交，TxHash: ", txhash)
		checkInTimes++
	}

	return nil
}

func main() {
	var jwt string
	var err error
	for i := 0; i < 5; i++ {
		jwt, err = login()
		if err != nil {
			continue
		}
		break
	}

	var currentNumber int
	for i := 0; i < 5; i++ {
		currentNumber, err = getCoinsNumber(jwt)
		if err != nil {
			continue
		}
		break
	}
	log.Println("当前积分: ", currentNumber, "\n目标积分: ", currentNumber+plusNumber)
	dailyCheckIn(jwt, currentNumber+plusNumber)
}
