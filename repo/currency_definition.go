package repo

import (
	"errors"
	"fmt"
	"runtime/debug"
	"strings"
)

const (
	Fiat   = "fiat"
	Crypto = "crypto"
)

var (
	ErrCurrencyCodeLengthInvalid       = errors.New("invalid length for currency code, must be three characters or four characters and begin with a 'T'")
	ErrCurrencyCodeTestSymbolInvalid   = errors.New("invalid test indicator for currency code, four characters must begin with a 'T'")
	ErrCurrencyDefinitionUndefined     = errors.New("currency definition is not defined")
	ErrCurrencyTypeInvalid             = errors.New("currency type must be crypto or fiat")
	ErrCurrencyDivisibilityNonPositive = errors.New("currency divisibility most be greater than zero")
	ErrDictionaryIndexMismatchedCode   = errors.New("dictionary index mismatched with definition currency code")

	validatedMainnetCurrencyDefs map[string]*CurrencyDefinition
	mainnetCurrencyDefinitions   = map[string]*CurrencyDefinition{
		// Crypto
		"BTC": {Name: "Bitcoin", Code: CurrencyCode("BTC"), CurrencyType: Crypto, Divisibility: 8},
		"BCH": {Name: "Bitcoin Cash", Code: CurrencyCode("BCH"), CurrencyType: Crypto, Divisibility: 8},
		"LTC": {Name: "Litecoin", Code: CurrencyCode("LTC"), CurrencyType: Crypto, Divisibility: 8},
		"ZEC": {Name: "Zcash", Code: CurrencyCode("ZEC"), CurrencyType: Crypto, Divisibility: 8},
		"ETH": {Name: "Ethereum", Code: CurrencyCode("ETH"), CurrencyType: Crypto, Divisibility: 18},

		// Fiat
		"AED": {Name: "UAE Dirham", Code: CurrencyCode("AED"), CurrencyType: Fiat, Divisibility: 2},
		"AFN": {Name: "Afghani", Code: CurrencyCode("AFN"), CurrencyType: Fiat, Divisibility: 2},
		"ALL": {Name: "Lek", Code: CurrencyCode("ALL"), CurrencyType: Fiat, Divisibility: 2},
		"AMD": {Name: "Armenian Dram", Code: CurrencyCode("AMD"), CurrencyType: Fiat, Divisibility: 2},
		"ANG": {Name: "Netherlands Antillean Guilder", Code: CurrencyCode("ANG"), CurrencyType: Fiat, Divisibility: 2},
		"AOA": {Name: "Kwanza", Code: CurrencyCode("AOA"), CurrencyType: Fiat, Divisibility: 2},
		"ARS": {Name: "Argentine Peso", Code: CurrencyCode("ARS"), CurrencyType: Fiat, Divisibility: 2},
		"AUD": {Name: "Australian Dollar", Code: CurrencyCode("AUD"), CurrencyType: Fiat, Divisibility: 2},
		"AWG": {Name: "Aruban Florin", Code: CurrencyCode("AWG"), CurrencyType: Fiat, Divisibility: 2},
		"AZN": {Name: "Azerbaijanian Manat", Code: CurrencyCode("AZN"), CurrencyType: Fiat, Divisibility: 2},
		"BAM": {Name: "Convertible Mark", Code: CurrencyCode("BAM"), CurrencyType: Fiat, Divisibility: 2},
		"BBD": {Name: "Barbados Dollar", Code: CurrencyCode("BBD"), CurrencyType: Fiat, Divisibility: 2},
		"BDT": {Name: "Taka", Code: CurrencyCode("BDT"), CurrencyType: Fiat, Divisibility: 2},
		"BGN": {Name: "Bulgarian Lev", Code: CurrencyCode("BGN"), CurrencyType: Fiat, Divisibility: 2},
		"BHD": {Name: "Bahraini Dinar", Code: CurrencyCode("BHD"), CurrencyType: Fiat, Divisibility: 2},
		"BIF": {Name: "Burundi Franc", Code: CurrencyCode("BIF"), CurrencyType: Fiat, Divisibility: 2},
		"BMD": {Name: "Bermudian Dollar", Code: CurrencyCode("BMD"), CurrencyType: Fiat, Divisibility: 2},
		"BND": {Name: "Brunei Dollar", Code: CurrencyCode("BND"), CurrencyType: Fiat, Divisibility: 2},
		"BOB": {Name: "Boliviano", Code: CurrencyCode("BOB"), CurrencyType: Fiat, Divisibility: 2},
		"BRL": {Name: "Brazilian Real", Code: CurrencyCode("BRL"), CurrencyType: Fiat, Divisibility: 2},
		"BSD": {Name: "Bahamian Dollar", Code: CurrencyCode("BSD"), CurrencyType: Fiat, Divisibility: 2},
		"BTN": {Name: "Ngultrum", Code: CurrencyCode("BTN"), CurrencyType: Fiat, Divisibility: 2},
		"BWP": {Name: "Pula", Code: CurrencyCode("BWP"), CurrencyType: Fiat, Divisibility: 2},
		"BYR": {Name: "Belarussian Ruble", Code: CurrencyCode("BYR"), CurrencyType: Fiat, Divisibility: 2},
		"BZD": {Name: "Belize Dollar", Code: CurrencyCode("BZD"), CurrencyType: Fiat, Divisibility: 2},
		"CAD": {Name: "Canadian Dollar", Code: CurrencyCode("CAD"), CurrencyType: Fiat, Divisibility: 2},
		"CDF": {Name: "Congolese Franc", Code: CurrencyCode("CDF"), CurrencyType: Fiat, Divisibility: 2},
		"CHF": {Name: "Swiss Franc", Code: CurrencyCode("CHF"), CurrencyType: Fiat, Divisibility: 2},
		"CLP": {Name: "Chilean Peso", Code: CurrencyCode("CLP"), CurrencyType: Fiat, Divisibility: 2},
		"CNY": {Name: "Yuan Renminbi", Code: CurrencyCode("CNY"), CurrencyType: Fiat, Divisibility: 2},
		"COP": {Name: "Colombian Peso", Code: CurrencyCode("COP"), CurrencyType: Fiat, Divisibility: 2},
		"CRC": {Name: "Costa Rican Colon", Code: CurrencyCode("CRC"), CurrencyType: Fiat, Divisibility: 2},
		"CUP": {Name: "Cuban Peso", Code: CurrencyCode("CUP"), CurrencyType: Fiat, Divisibility: 2},
		"CVE": {Name: "Cabo Verde Escudo", Code: CurrencyCode("CVE"), CurrencyType: Fiat, Divisibility: 2},
		"CZK": {Name: "Czech Koruna", Code: CurrencyCode("CZK"), CurrencyType: Fiat, Divisibility: 2},
		"DJF": {Name: "Djibouti Franc", Code: CurrencyCode("DJF"), CurrencyType: Fiat, Divisibility: 2},
		"DKK": {Name: "Danish Krone", Code: CurrencyCode("DKK"), CurrencyType: Fiat, Divisibility: 2},
		"DOP": {Name: "Dominican Peso", Code: CurrencyCode("DOP"), CurrencyType: Fiat, Divisibility: 2},
		"DZD": {Name: "Algerian Dinar", Code: CurrencyCode("DZD"), CurrencyType: Fiat, Divisibility: 2},
		"EGP": {Name: "Egyptian Pound", Code: CurrencyCode("EGP"), CurrencyType: Fiat, Divisibility: 2},
		"ERN": {Name: "Nakfa", Code: CurrencyCode("ERN"), CurrencyType: Fiat, Divisibility: 2},
		"ETB": {Name: "Ethiopian Birr", Code: CurrencyCode("ETB"), CurrencyType: Fiat, Divisibility: 2},
		"EUR": {Name: "Euro", Code: CurrencyCode("EUR"), CurrencyType: Fiat, Divisibility: 2},
		"FJD": {Name: "Fiji Dollar", Code: CurrencyCode("FJD"), CurrencyType: Fiat, Divisibility: 2},
		"FKP": {Name: "Falkland Islands Pound", Code: CurrencyCode("FKP"), CurrencyType: Fiat, Divisibility: 2},
		"GBP": {Name: "Pound Sterling", Code: CurrencyCode("GBP"), CurrencyType: Fiat, Divisibility: 2},
		"GEL": {Name: "Lari", Code: CurrencyCode("GEL"), CurrencyType: Fiat, Divisibility: 2},
		"GHS": {Name: "Ghana Cedi", Code: CurrencyCode("GHS"), CurrencyType: Fiat, Divisibility: 2},
		"GIP": {Name: "Gibraltar Pound", Code: CurrencyCode("GIP"), CurrencyType: Fiat, Divisibility: 2},
		"GMD": {Name: "Dalasi", Code: CurrencyCode("GMD"), CurrencyType: Fiat, Divisibility: 2},
		"GNF": {Name: "Guinea Franc", Code: CurrencyCode("GNF"), CurrencyType: Fiat, Divisibility: 2},
		"GTQ": {Name: "Quetzal", Code: CurrencyCode("GTQ"), CurrencyType: Fiat, Divisibility: 2},
		"GYD": {Name: "Guyana Dollar", Code: CurrencyCode("GYD"), CurrencyType: Fiat, Divisibility: 2},
		"HKD": {Name: "Hong Kong Dollar", Code: CurrencyCode("HKD"), CurrencyType: Fiat, Divisibility: 2},
		"HNL": {Name: "Lempira", Code: CurrencyCode("HNL"), CurrencyType: Fiat, Divisibility: 2},
		"HRK": {Name: "Kuna", Code: CurrencyCode("HRK"), CurrencyType: Fiat, Divisibility: 2},
		"HTG": {Name: "Gourde", Code: CurrencyCode("HTG"), CurrencyType: Fiat, Divisibility: 2},
		"HUF": {Name: "Forint", Code: CurrencyCode("HUF"), CurrencyType: Fiat, Divisibility: 2},
		"IDR": {Name: "Rupiah", Code: CurrencyCode("IDR"), CurrencyType: Fiat, Divisibility: 2},
		"ILS": {Name: "New Israeli Sheqel", Code: CurrencyCode("ILS"), CurrencyType: Fiat, Divisibility: 2},
		"INR": {Name: "Indian Rupee", Code: CurrencyCode("INR"), CurrencyType: Fiat, Divisibility: 2},
		"IQD": {Name: "Iraqi Dinar", Code: CurrencyCode("IQD"), CurrencyType: Fiat, Divisibility: 2},
		"IRR": {Name: "Iranian Rial", Code: CurrencyCode("IRR"), CurrencyType: Fiat, Divisibility: 2},
		"ISK": {Name: "Iceland Krona", Code: CurrencyCode("ISK"), CurrencyType: Fiat, Divisibility: 2},
		"JMD": {Name: "Jamaican Dollar", Code: CurrencyCode("JMD"), CurrencyType: Fiat, Divisibility: 2},
		"JOD": {Name: "Jordanian Dinar", Code: CurrencyCode("JOD"), CurrencyType: Fiat, Divisibility: 2},
		"JPY": {Name: "Yen", Code: CurrencyCode("JPY"), CurrencyType: Fiat, Divisibility: 2},
		"KES": {Name: "Kenyan Shilling", Code: CurrencyCode("KES"), CurrencyType: Fiat, Divisibility: 2},
		"KGS": {Name: "Som", Code: CurrencyCode("KGS"), CurrencyType: Fiat, Divisibility: 2},
		"KHR": {Name: "Riel", Code: CurrencyCode("KHR"), CurrencyType: Fiat, Divisibility: 2},
		"KMF": {Name: "Comoro Franc", Code: CurrencyCode("KMF"), CurrencyType: Fiat, Divisibility: 2},
		"KPW": {Name: "North Korean Won", Code: CurrencyCode("KPW"), CurrencyType: Fiat, Divisibility: 2},
		"KRW": {Name: "Won", Code: CurrencyCode("KRW"), CurrencyType: Fiat, Divisibility: 2},
		"KWD": {Name: "Kuwaiti Dinar", Code: CurrencyCode("KWD"), CurrencyType: Fiat, Divisibility: 2},
		"KYD": {Name: "Cayman Islands Dollar", Code: CurrencyCode("KYD"), CurrencyType: Fiat, Divisibility: 2},
		"KZT": {Name: "Tenge", Code: CurrencyCode("KZT"), CurrencyType: Fiat, Divisibility: 2},
		"LAK": {Name: "Kip", Code: CurrencyCode("LAK"), CurrencyType: Fiat, Divisibility: 2},
		"LBP": {Name: "Lebanese Pound", Code: CurrencyCode("LBP"), CurrencyType: Fiat, Divisibility: 2},
		"LKR": {Name: "Sri Lanka Rupee", Code: CurrencyCode("LKR"), CurrencyType: Fiat, Divisibility: 2},
		"LRD": {Name: "Liberian Dollar", Code: CurrencyCode("LRD"), CurrencyType: Fiat, Divisibility: 2},
		"LSL": {Name: "Loti", Code: CurrencyCode("LSL"), CurrencyType: Fiat, Divisibility: 2},
		"LYD": {Name: "Libyan Dinar", Code: CurrencyCode("LYD"), CurrencyType: Fiat, Divisibility: 2},
		"MAD": {Name: "Moroccan Dirham", Code: CurrencyCode("MAD"), CurrencyType: Fiat, Divisibility: 2},
		"MDL": {Name: "Moldovan Leu", Code: CurrencyCode("MDL"), CurrencyType: Fiat, Divisibility: 2},
		"MGA": {Name: "Malagasy Ariary", Code: CurrencyCode("MGA"), CurrencyType: Fiat, Divisibility: 2},
		"MKD": {Name: "Denar", Code: CurrencyCode("MKD"), CurrencyType: Fiat, Divisibility: 2},
		"MMK": {Name: "Kyat", Code: CurrencyCode("MMK"), CurrencyType: Fiat, Divisibility: 2},
		"MNT": {Name: "Tugrik", Code: CurrencyCode("MNT"), CurrencyType: Fiat, Divisibility: 2},
		"MOP": {Name: "Pataca", Code: CurrencyCode("MOP"), CurrencyType: Fiat, Divisibility: 2},
		"MRO": {Name: "Ouguiya", Code: CurrencyCode("MRO"), CurrencyType: Fiat, Divisibility: 2},
		"MUR": {Name: "Mauritius Rupee", Code: CurrencyCode("MUR"), CurrencyType: Fiat, Divisibility: 2},
		"MVR": {Name: "Rufiyaa", Code: CurrencyCode("MVR"), CurrencyType: Fiat, Divisibility: 2},
		"MWK": {Name: "Kwacha", Code: CurrencyCode("MWK"), CurrencyType: Fiat, Divisibility: 2},
		"MXN": {Name: "Mexican Peso", Code: CurrencyCode("MXN"), CurrencyType: Fiat, Divisibility: 2},
		"MYR": {Name: "Malaysian Ringgit", Code: CurrencyCode("MYR"), CurrencyType: Fiat, Divisibility: 2},
		"MZN": {Name: "Mozambique Metical", Code: CurrencyCode("MZN"), CurrencyType: Fiat, Divisibility: 2},
		"NAD": {Name: "Namibia Dollar", Code: CurrencyCode("NAD"), CurrencyType: Fiat, Divisibility: 2},
		"NGN": {Name: "Naira", Code: CurrencyCode("NGN"), CurrencyType: Fiat, Divisibility: 2},
		"NIO": {Name: "Cordoba Oro", Code: CurrencyCode("NIO"), CurrencyType: Fiat, Divisibility: 2},
		"NOK": {Name: "Norwegian Krone", Code: CurrencyCode("NOK"), CurrencyType: Fiat, Divisibility: 2},
		"NPR": {Name: "Nepalese Rupee", Code: CurrencyCode("NPR"), CurrencyType: Fiat, Divisibility: 2},
		"NZD": {Name: "New Zealand Dollar", Code: CurrencyCode("NZD"), CurrencyType: Fiat, Divisibility: 2},
		"OMR": {Name: "Rial Omani", Code: CurrencyCode("OMR"), CurrencyType: Fiat, Divisibility: 2},
		"PAB": {Name: "Balboa", Code: CurrencyCode("PAB"), CurrencyType: Fiat, Divisibility: 2},
		"PEN": {Name: "Nuevo Sol", Code: CurrencyCode("PEN"), CurrencyType: Fiat, Divisibility: 2},
		"PGK": {Name: "Kina", Code: CurrencyCode("PGK"), CurrencyType: Fiat, Divisibility: 2},
		"PHP": {Name: "Philippine Peso", Code: CurrencyCode("PHP"), CurrencyType: Fiat, Divisibility: 2},
		"PKR": {Name: "Pakistan Rupee", Code: CurrencyCode("PKR"), CurrencyType: Fiat, Divisibility: 2},
		"PLN": {Name: "Zloty", Code: CurrencyCode("PLN"), CurrencyType: Fiat, Divisibility: 2},
		"PYG": {Name: "Guarani", Code: CurrencyCode("PYG"), CurrencyType: Fiat, Divisibility: 2},
		"QAR": {Name: "Qatari Rial", Code: CurrencyCode("QAR"), CurrencyType: Fiat, Divisibility: 2},
		"RON": {Name: "Romanian Leu", Code: CurrencyCode("RON"), CurrencyType: Fiat, Divisibility: 2},
		"RSD": {Name: "Serbian Dinar", Code: CurrencyCode("RSD"), CurrencyType: Fiat, Divisibility: 2},
		"RUB": {Name: "Russian Ruble", Code: CurrencyCode("RUB"), CurrencyType: Fiat, Divisibility: 2},
		"RWF": {Name: "Rwanda Franc", Code: CurrencyCode("RWF"), CurrencyType: Fiat, Divisibility: 2},
		"SAR": {Name: "Saudi Riyal", Code: CurrencyCode("SAR"), CurrencyType: Fiat, Divisibility: 2},
		"SBD": {Name: "Solomon Islands Dollar", Code: CurrencyCode("SBD"), CurrencyType: Fiat, Divisibility: 2},
		"SCR": {Name: "Seychelles Rupee", Code: CurrencyCode("SCR"), CurrencyType: Fiat, Divisibility: 2},
		"SDG": {Name: "Sudanese Pound", Code: CurrencyCode("SDG"), CurrencyType: Fiat, Divisibility: 2},
		"SEK": {Name: "Swedish Krona", Code: CurrencyCode("SEK"), CurrencyType: Fiat, Divisibility: 2},
		"SGD": {Name: "Singapore Dollar", Code: CurrencyCode("SGD"), CurrencyType: Fiat, Divisibility: 2},
		"SHP": {Name: "Saint Helena Pound", Code: CurrencyCode("SHP"), CurrencyType: Fiat, Divisibility: 2},
		"SLL": {Name: "Leone", Code: CurrencyCode("SLL"), CurrencyType: Fiat, Divisibility: 2},
		"SOS": {Name: "Somali Shilling", Code: CurrencyCode("SOS"), CurrencyType: Fiat, Divisibility: 2},
		"SRD": {Name: "Surinam Dollar", Code: CurrencyCode("SRD"), CurrencyType: Fiat, Divisibility: 2},
		"SSP": {Name: "South Sudanese Pound", Code: CurrencyCode("SSP"), CurrencyType: Fiat, Divisibility: 2},
		"STD": {Name: "Dobra", Code: CurrencyCode("STD"), CurrencyType: Fiat, Divisibility: 2},
		"SVC": {Name: "El Salvador Colon", Code: CurrencyCode("SVC"), CurrencyType: Fiat, Divisibility: 2},
		"SYP": {Name: "Syrian Pound", Code: CurrencyCode("SYP"), CurrencyType: Fiat, Divisibility: 2},
		"SZL": {Name: "Lilangeni", Code: CurrencyCode("SZL"), CurrencyType: Fiat, Divisibility: 2},
		"THB": {Name: "Baht", Code: CurrencyCode("THB"), CurrencyType: Fiat, Divisibility: 2},
		"TJS": {Name: "Somoni", Code: CurrencyCode("TJS"), CurrencyType: Fiat, Divisibility: 2},
		"TMT": {Name: "Turkmenistan New Manat", Code: CurrencyCode("TMT"), CurrencyType: Fiat, Divisibility: 2},
		"TND": {Name: "Tunisian Dinar", Code: CurrencyCode("TND"), CurrencyType: Fiat, Divisibility: 2},
		"TOP": {Name: "Pa\"anga", Code: CurrencyCode("TOP"), CurrencyType: Fiat, Divisibility: 2},
		"TRY": {Name: "Turkish Lira", Code: CurrencyCode("TRY"), CurrencyType: Fiat, Divisibility: 2},
		"TTD": {Name: "Trinidad and Tobago Dollar", Code: CurrencyCode("TTD"), CurrencyType: Fiat, Divisibility: 2},
		"TWD": {Name: "New Taiwan Dollar", Code: CurrencyCode("TWD"), CurrencyType: Fiat, Divisibility: 2},
		"TZS": {Name: "Tanzanian Shilling", Code: CurrencyCode("TZS"), CurrencyType: Fiat, Divisibility: 2},
		"UAH": {Name: "Hryvnia", Code: CurrencyCode("UAH"), CurrencyType: Fiat, Divisibility: 2},
		"UGX": {Name: "Uganda Shilling", Code: CurrencyCode("UGX"), CurrencyType: Fiat, Divisibility: 2},
		"USD": {Name: "United States Dollar", Code: CurrencyCode("USD"), CurrencyType: Fiat, Divisibility: 2},
		"UYU": {Name: "Peso Uruguayo", Code: CurrencyCode("UYU"), CurrencyType: Fiat, Divisibility: 2},
		"UZS": {Name: "Uzbekistan Sum", Code: CurrencyCode("UZS"), CurrencyType: Fiat, Divisibility: 2},
		"VEF": {Name: "Bolivar", Code: CurrencyCode("VEF"), CurrencyType: Fiat, Divisibility: 2},
		"VND": {Name: "Dong", Code: CurrencyCode("VND"), CurrencyType: Fiat, Divisibility: 2},
		"VUV": {Name: "Vatu", Code: CurrencyCode("VUV"), CurrencyType: Fiat, Divisibility: 2},
		"WST": {Name: "Tala", Code: CurrencyCode("WST"), CurrencyType: Fiat, Divisibility: 2},
		"XAF": {Name: "CFA Franc BEAC", Code: CurrencyCode("XAF"), CurrencyType: Fiat, Divisibility: 2},
		"XCD": {Name: "East Caribbean Dollar", Code: CurrencyCode("XCD"), CurrencyType: Fiat, Divisibility: 2},
		"XOF": {Name: "CFA Franc BCEAO", Code: CurrencyCode("XOF"), CurrencyType: Fiat, Divisibility: 2},
		"XPF": {Name: "CFP Franc", Code: CurrencyCode("XPF"), CurrencyType: Fiat, Divisibility: 2},
		"XSU": {Name: "Sucre", Code: CurrencyCode("XSU"), CurrencyType: Fiat, Divisibility: 2},
		"YER": {Name: "Yemeni Rial", Code: CurrencyCode("YER"), CurrencyType: Fiat, Divisibility: 2},
		"ZAR": {Name: "Rand", Code: CurrencyCode("ZAR"), CurrencyType: Fiat, Divisibility: 2},
		"ZMW": {Name: "Zambian Kwacha", Code: CurrencyCode("ZMW"), CurrencyType: Fiat, Divisibility: 2},
		"ZWL": {Name: "Zimbabwe Dollar", Code: CurrencyCode("ZWL"), CurrencyType: Fiat, Divisibility: 2},
	}
)

type (
	// CurrencyCode is a string-based currency symbol
	CurrencyCode string
	// CurrencyDefinition defines the characteristics of a currency
	CurrencyDefinition struct {
		Name         string
		Code         CurrencyCode
		Divisibility uint
		CurrencyType string
	}
	// CurrencyDictionaryProcessingError represents a list of errors after
	// processing a CurrencyDictionary
	CurrencyDictionaryProcessingError map[string]error
	// CurrencyDictionary represents a collection of CurrencyDefinitions keyed
	// by their CurrencyCode in string form
	CurrencyDictionary map[string]*CurrencyDefinition
)

// String returns a readable representation of CurrencyCode
func (c CurrencyCode) String() string { return string(c) }

// String returns a readable representation of CurrencyDefinition
func (c *CurrencyDefinition) String() string {
	if c == nil {
		log.Errorf("returning nil CurrencyCode, please report this bug")
		debug.PrintStack()
		return "nil"
	}
	return c.Code.String()
}

// CurrencyCode returns the CurrencyCode of the definition
func (c *CurrencyDefinition) CurrencyCode() *CurrencyCode { return &c.Code }

func (c CurrencyDictionaryProcessingError) Error() string {
	return fmt.Sprintf("dictionary contains %d invalid definitions", len(c))
}

// LoadCurrencyDefinitions returns the mainnet CurrencyDictionary singleton which
// references all pre-defined mainnet CurrencyDefinitions
func LoadCurrencyDefinitions() CurrencyDictionary {
	if validatedMainnetCurrencyDefs == nil {
		dict, _ := NewCurrencyDictionary(mainnetCurrencyDefinitions)
		validatedMainnetCurrencyDefs = dict
	}
	return validatedMainnetCurrencyDefs
}

// NewCurrencyDictionary returns a CurrencyDictionary for managing CurrencyDefinitions
func NewCurrencyDictionary(defs map[string]*CurrencyDefinition) (CurrencyDictionary, error) {
	var errs = make(CurrencyDictionaryProcessingError)
	for code, def := range defs {
		if err := def.Valid(); err != nil {
			errs[code] = err
			continue
		}
		if code != def.Code.String() {
			errs[code] = ErrDictionaryIndexMismatchedCode
			continue
		}
	}
	if len(errs) > 0 {
		return nil, errs
	}
	return CurrencyDictionary(defs), nil
}

func (c *CurrencyDefinition) Valid() error {
	if c == nil {
		return ErrCurrencyDefinitionUndefined
	}
	if len(c.Code) < CurrencyCodeValidMinimumLength || len(c.Code) > CurrencyCodeValidMaximumLength {
		return ErrCurrencyCodeLengthInvalid
	}
	if len(c.Code) == 4 && strings.Index(strings.ToLower(string(c.Code)), "t") != 0 {
		return ErrCurrencyCodeTestSymbolInvalid
	}
	if c.CurrencyType != Crypto && c.CurrencyType != Fiat {
		return ErrCurrencyTypeInvalid
	}
	if c.Divisibility == 0 {
		return ErrCurrencyDivisibilityNonPositive
	}

	return nil
}

// Equal indicates if the receiver and other have the same code
// and divisibility
func (c *CurrencyDefinition) Equal(other *CurrencyDefinition) bool {
	if c == nil || other == nil {
		return false
	}
	if c.Code != other.Code {
		return false
	}
	if c.Divisibility != other.Divisibility {
		return false
	}
	if c.CurrencyType != other.CurrencyType {
		return false
	}
	return true
}

// LookupCurrencyDefinition returns the CurrencyDefinition out of the loaded dictionary.
// Lookup normalizes the code before lookup and recommends using CurrencyDefinition.Code
// from the response as a normalized code.
func (c CurrencyDictionary) Lookup(code string) (*CurrencyDefinition, error) {
	var (
		upcase    = strings.ToUpper(code)
		isTestnet = strings.HasPrefix(upcase, "T")

		def *CurrencyDefinition
		ok  bool
	)
	if isTestnet {
		def, ok = c[strings.TrimPrefix(upcase, "T")]
	} else {
		def, ok = c[upcase]
	}
	if !ok {
		return nil, ErrCurrencyDefinitionUndefined
	}
	if isTestnet {
		return NewTestnetDefinition(def), nil
	}
	return def, nil
}

func NewTestnetDefinition(def *CurrencyDefinition) *CurrencyDefinition {
	return &CurrencyDefinition{
		Name:         def.Name,
		Code:         CurrencyCode(fmt.Sprintf("T%s", def.Code)),
		Divisibility: def.Divisibility,
		CurrencyType: def.CurrencyType,
	}
}
