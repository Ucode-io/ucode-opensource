package helper

import (
	"context"
	"crypto/rand"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"math"
	"math/big"
	mr "math/rand"
	"net/http"
	"strconv"
	"strings"
	"time"
	"github.com/Ucode-io/ucode-opensource/services/company/config"
	"unicode"
	"unicode/utf8"

	"google.golang.org/protobuf/encoding/protojson"
	"google.golang.org/protobuf/types/known/structpb"
)

var (
	digits       = "0123456789"
	specials     = "~=+%^*/()[]{}/!@#$?|"
	upperLetters = "ABCDEFGHIJKLMNOPQRSTUVWXYZ"
	lowerLetters = "abcdefghijklmnopqrstuvwxyz"
)

func ReplaceQueryParams(namedQuery string, params map[string]any) (string, []any) {
	var (
		i    int = 1
		args []any
	)

	for k, v := range params {
		if k != "" && strings.Contains(namedQuery, ":"+k) {
			namedQuery = strings.ReplaceAll(namedQuery, ":"+k, "$"+strconv.Itoa(i))
			args = append(args, v)
			i++
		}
	}

	return namedQuery, args
}

func ReplaceSQL(old, searchPattern string) string {
	tmpCount := strings.Count(old, searchPattern)
	for m := 1; m <= tmpCount; m++ {
		old = strings.Replace(old, searchPattern, "$"+strconv.Itoa(m), 1)
	}
	return old
}

func ConvertMapToStruct(inputMap map[string]any) (*structpb.Struct, error) {
	marshledInputMap, err := json.Marshal(inputMap)
	outputStruct := &structpb.Struct{}
	if err != nil {
		return outputStruct, err
	}
	err = protojson.Unmarshal(marshledInputMap, outputStruct)

	return outputStruct, err
}

func ConvertRequestToSturct(inputRequest any) (*structpb.Struct, error) {
	marshelledInputInterface, err := json.Marshal(inputRequest)
	outputStruct := &structpb.Struct{}
	if err != nil {
		return outputStruct, err
	}
	err = protojson.Unmarshal(marshelledInputInterface, outputStruct)
	return outputStruct, err
}

func ConvertStructToResponse(inputStruct *structpb.Struct) (map[string]any, error) {
	marshelledInputStruct, err := protojson.Marshal(inputStruct)
	outputMap := make(map[string]any, 0)
	if err != nil {
		return nil, err
	}
	err = json.Unmarshal(marshelledInputStruct, &outputMap)
	return outputMap, err
}

func ConverPhoneNumberToMongoPhoneFormat(input string) string {
	//input +998995677777
	input = input[4:]
	// input  = 995677777
	changedEl := input[:2]
	input = "(" + changedEl + ") " + input[2:5] + "-" + input[5:7] + "-" + input[7:]
	// input = (99) 567-77-77
	return input
}

func GeneratePassword(length int, digit bool, upperLetter bool, lowerLetter bool, special bool) string {
	var (
		all string
	)

	if upperLetter {
		all += upperLetters
	}

	if lowerLetter {
		all += lowerLetters
	}

	if digit {
		all += digits
	}

	if special {
		all += specials
	}

	buf := make([]byte, length)

	for i := 0; i < length; i++ {
		buf[i] = all[cryptoRandSecure(int64(len(all)))]
	}

	return string(buf)
}

func cryptoRandSecure(max int64) int64 {
	nBig, err := rand.Int(rand.Reader, big.NewInt(max))
	if err != nil {
		log.Println(err)
	}
	return nBig.Int64()
}

func Min(n1, n2 int) int {
	if n1 < n2 {
		return n1
	}
	return n2
}

func Max(n1, n2 int) int {
	if n1 > n2 {
		return n1
	}
	return n2
}

func GenerateRandomNumber(numDigits int) int {
	if numDigits < 1 {
		numDigits = 6
	}

	mr.New(mr.NewSource(time.Now().UnixNano()))

	min := int(math.Pow10(numDigits - 1))
	max := int(math.Pow10(numDigits)) - 1

	return mr.Intn(max-min+1) + min
}

type ExchangeRate struct {
	CbuDate string `json:"CbuDate"`
	Code    string `json:"Code"`
	Ccy     string `json:"Ccy"`
	Rate    string `json:"Rate"`
}

func FetchCurrencyRate(ctx context.Context, currency, date string) (float64, error) {
	url := fmt.Sprintf("%s/%s/%s/", config.CBUURL, currency, date)

	client := &http.Client{Timeout: 10 * time.Second}

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return 0, err
	}

	// Send request
	resp, err := client.Do(req)
	if err != nil {
		return 0, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return 0, fmt.Errorf("unexpected status code: %d", resp.StatusCode)
	}

	var rates []ExchangeRate
	err = json.NewDecoder(resp.Body).Decode(&rates)
	if err != nil {
		return 0, err
	}

	if len(rates) == 0 {
		return 0, errors.New("no currency")
	}

	rate, err := strconv.ParseFloat(rates[0].Rate, 64)
	if err != nil {
		return 0, err
	}

	return rate, nil
}

func RoundToTwoDecimals(f float64, decimal int) float64 {
	return math.Round(f*math.Pow10(decimal)) / math.Pow10(decimal)
}

func GenerateRandomPassword(length int) (string, error) {
	const charset = "ABCDEFGHIJKLMNOPQRSTUVWXYZ" +
		"abcdefghijklmnopqrstuvwxyz" +
		"0123456789"

	charsetLength := big.NewInt(int64(len(charset)))
	password := make([]byte, length)

	for i := range password {
		n, err := rand.Int(rand.Reader, charsetLength)
		if err != nil {
			return "", errors.New("failed to generate random password: " + err.Error())
		}
		password[i] = charset[n.Int64()]
	}

	return string(password), nil
}

func GenerateRandomWord(length int) (string, error) {
	const charset = "ABCDEFGHIJKLMNOPQRSTUVWXYZ" +
		"abcdefghijklmnopqrstuvwxyz"

	charsetLength := big.NewInt(int64(len(charset)))
	word := make([]byte, length)

	for i := range word {
		n, err := rand.Int(rand.Reader, charsetLength)
		if err != nil {
			return "", errors.New("failed to generate random word: " + err.Error())
		}
		word[i] = charset[n.Int64()]
	}

	return string(word), nil
}

func CapitalizeFirst(s string) string {
	if s == "" {
		return s
	}
	r, size := utf8.DecodeRuneInString(s)
	return string(unicode.ToUpper(r)) + s[size:]
}
