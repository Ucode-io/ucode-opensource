package models

type CreateProjectCardRequest struct {
	Pan    string `json:"pan"`
	Expire string `json:"expire"`
}

type CreateCardResponse struct {
	Id     int         `json:"id"`
	Result *CardResult `json:"result"`
	Error  *Error      `json:"error"`
}

type CardResult struct {
	Card Card `json:"card"`
}

type Card struct {
	Number     string `json:"number"`
	Expire     string `json:"expire"`
	PaymeToken string `json:"token"`
	Recurrent  bool   `json:"recurrent"`
	Verify     bool   `json:"verify"`
	Type       string `json:"type"`
}

type CreateCardError struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
	Data    *Data  `json:"data"`
}

type Data struct {
	Message *Message `json:"message"`
}

type Message struct {
	Ru string `json:"ru"`
	En string `json:"en"`
	Uz string `json:"Uz"`
}

type Error struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
	Data    string `json:"data"`
}

type GetVerifyCodeResponse struct {
	Result *VerifyCodeResult `json:"result"`
	Error  *Error            `json:"error"`
}

type VerifyCodeResult struct {
	Sent       bool   `json:"sent"`
	Phone      string `json:"phone"`
	Wait       int64  `json:"wait"`
	PaymeToken string `json:"token"`
}

type VerifyCardRequest struct {
	PaymeToken string `json:"token"`
	Code       string `json:"code"`
}

type ReceiptCreateResponse struct {
	ID     int    `json:"id"`
	Error  *Error `json:"error"`
	Result struct {
		Receipt Receipt `json:"receipt"`
	} `json:"result"`
}

type Receipt struct {
	ID           string    `json:"_id"`
	CreateTime   int64     `json:"create_time"`
	PayTime      int64     `json:"pay_time"`
	CancelTime   int64     `json:"cancel_time"`
	State        int       `json:"state"`
	Type         int       `json:"type"`
	External     bool      `json:"external"`
	Operation    int       `json:"operation"`
	Error        *string   `json:"error"`
	Description  string    `json:"description"`
	Detail       *string   `json:"detail"`
	Amount       int       `json:"amount"`
	Currency     int       `json:"currency"`
	Commission   int       `json:"commission"`
	Account      []Account `json:"account"`
	Card         Card      `json:"card"`
	Creator      *string   `json:"creator"`
	Payer        Payer     `json:"payer"`
	Merchant     Merchant  `json:"merchant"`
	Meta         Meta      `json:"meta"`
	ProcessingID int       `json:"processing_id"`
}

type Account struct {
	Name  string `json:"name"`
	Title string `json:"title"`
	Value string `json:"value"`
	Main  bool   `json:"main,omitempty"`
}

type Payer struct {
	Phone string `json:"phone"`
}

type Merchant struct {
	ID           string  `json:"_id"`
	Name         string  `json:"name"`
	Organization string  `json:"organization"`
	Address      string  `json:"address"`
	BusinessID   string  `json:"business_id"`
	Epos         Epos    `json:"epos"`
	Restrictions *string `json:"restrictions"`
	Date         int64   `json:"date"`
	Logo         *string `json:"logo"`
	Type         string  `json:"type"`
	Terms        *string `json:"terms"`
}

type Epos struct {
	MerchantID string `json:"merchantId"`
	TerminalID string `json:"terminalId"`
}

type Meta struct {
	Source string `json:"source"`
	Owner  string `json:"owner"`
	Host   string `json:"host"`
}

type CardRemoveResponse struct {
	JSONRPC string `json:"jsonrpc"`
	ID      int    `json:"id"`
	Result  struct {
		Success bool `json:"success"`
	} `json:"result"`
	Error *Error `json:"error"`
}
