package models

type Language struct {
	Id         string `json:"id"`
	Name       string `json:"name"`
	ShortName  string `json:"short_name"`
	NativeName string `json:"native_name"`
}

type Environment struct {
	Id           string         `json:"id"`
	ProjectId    string         `json:"project_id"`
	Name         string         `json:"name"`
	DisplayColor string         `json:"display_color"`
	Description  string         `json:"description"`
	Data         map[string]any `json:"data"`
}

type Currency struct {
	Id            string `json:"id"`
	Symbol        string `json:"symbol"`
	Name          string `json:"name"`
	SymbolNative  string `json:"symbol_native"`
	DecimalDigits int    `json:"decimal_digits"`
	Rounding      int    `json:"rounding"`
	Code          string `json:"code"`
	NamePlural    string `json:"name_plural"`
}

type Timezone struct {
	Id   string `json:"id"`
	Name string `json:"name"`
	Text string `json:"text"`
}

type ListLanguage struct {
	Language []*Language `json:"language"`
	Count    int         `json:"count"`
}

type ListCurrency struct {
	Currency []*Currency `json:"currency"`
	Count    int         `json:"count"`
}

type ListTimezone struct {
	Timezone []*Timezone `json:"timezone"`
	Count    int         `json:"count"`
}
