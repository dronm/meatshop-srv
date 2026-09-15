package main

import (
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"net/url"
	"os"
	"sort"
	"strconv"
	"strings"
	"time"
)

type maxUser struct {
	ID           int64   `json:"id"`
	FirstName    string  `json:"first_name"`
	LastName     string  `json:"last_name"`
	Username     *string `json:"username"`
	LanguageCode string  `json:"language_code"`
	PhotoURL     *string `json:"photo_url"`
}

type maxChat struct {
	ID   int64  `json:"id"`
	Type string `json:"type"`
}

func hmacSHA256(key []byte, data []byte) []byte {
	mac := hmac.New(sha256.New, key)
	_, _ = mac.Write(data)
	return mac.Sum(nil)
}

func randomUUID() (string, error) {
	b := make([]byte, 16)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}

	b[6] = (b[6] & 0x0f) | 0x40
	b[8] = (b[8] & 0x3f) | 0x80

	return fmt.Sprintf(
		"%08x-%04x-%04x-%04x-%012x",
		b[0:4],
		b[4:6],
		b[6:8],
		b[8:10],
		b[10:16],
	), nil
}

func generateInitData(
	botToken string,
	user maxUser,
	chat maxChat,
) (string, error) {
	queryID, err := randomUUID()
	if err != nil {
		return "", err
	}

	userJSON, err := json.Marshal(user)
	if err != nil {
		return "", err
	}

	chatJSON, err := json.Marshal(chat)
	if err != nil {
		return "", err
	}

	params := map[string]string{
		"auth_date": strconv.FormatInt(time.Now().Unix(), 10),
		"chat":      string(chatJSON),
		"query_id":  queryID,
		"user":      string(userJSON),
	}

	keys := make([]string, 0, len(params))
	for key := range params {
		keys = append(keys, key)
	}
	sort.Strings(keys)

	launchParts := make([]string, 0, len(keys))
	for _, key := range keys {
		launchParts = append(
			launchParts,
			key+"="+params[key],
		)
	}

	launchParams := strings.Join(launchParts, "\n")

	secretKey := hmacSHA256(
		[]byte("WebAppData"),
		[]byte(botToken),
	)

	signature := hmacSHA256(
		secretKey,
		[]byte(launchParams),
	)

	hash := hex.EncodeToString(signature)

	values := url.Values{}
	for key, value := range params {
		values.Set(key, value)
	}
	values.Set("hash", hash)

	return values.Encode(), nil
}

func main() {
	userID := flag.Int64(
		"user-id",
		990000000001,
		"Fake MAX user ID",
	)
	firstName := flag.String(
		"first-name",
		"Test",
		"First name",
	)
	lastName := flag.String(
		"last-name",
		"User",
		"Last name",
	)
	username := flag.String(
		"username",
		"test_user",
		"Username",
	)

	flag.Parse()

	botToken := os.Getenv("MAX_BOT_TOKEN")
	if botToken == "" {
		log.Fatal("MAX_BOT_TOKEN is required")
	}

	usernameValue := *username

	user := maxUser{
		ID:           *userID,
		FirstName:    *firstName,
		LastName:     *lastName,
		Username:     &usernameValue,
		LanguageCode: "ru",
		PhotoURL:     nil,
	}

	chat := maxChat{
		ID:   *userID,
		Type: "DIALOG",
	}

	initData, err := generateInitData(
		botToken,
		user,
		chat,
	)
	if err != nil {
		log.Fatal(err)
	}

	fmt.Println(initData)
}
