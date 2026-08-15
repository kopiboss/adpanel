package helpers

import (
	"fmt"
	"math"
	"strings"
)

// currencyMeta menyimpan simbol dan metadata per currency code.
type currencyMeta struct {
	Symbol   string
	Decimals int
	Divisor  float64 // berapa unit terkecil per 1 unit utama (Meta API)
}

// Meta API menyimpan budget & spend dalam unit terkecil mata uang.
// USD → sen (divisor=100), IDR → rupiah (tidak ada desimal, divisor=1),
// VND, JPY, KRW dll juga divisor=1 karena tidak ada sub-unit.
var currencyMap = map[string]currencyMeta{
	"IDR": {"Rp", 0, 1},
	"USD": {"$", 2, 100},
	"SGD": {"S$", 2, 100},
	"MYR": {"RM", 2, 100},
	"AUD": {"A$", 2, 100},
	"EUR": {"€", 2, 100},
	"GBP": {"£", 2, 100},
	"THB": {"฿", 2, 100},
	"PHP": {"₱", 2, 100},
	"VND": {"₫", 0, 1},
	"JPY": {"¥", 0, 1},
	"KRW": {"₩", 0, 1},
	"TWD": {"NT$", 0, 1},
	"HKD": {"HK$", 2, 100},
	"CNY": {"¥", 2, 100},
	"INR": {"₹", 2, 100},
	"BRL": {"R$", 2, 100},
	"MXN": {"MX$", 2, 100},
	"CAD": {"C$", 2, 100},
	"NZD": {"NZ$", 2, 100},
	"CHF": {"CHF", 2, 100},
	"SEK": {"kr", 2, 100},
	"NOK": {"kr", 2, 100},
	"DKK": {"kr", 2, 100},
	"CZK": {"Kč", 2, 100},
	"PLN": {"zł", 2, 100},
	"HUF": {"Ft", 0, 1},
	"RON": {"lei", 2, 100},
	"ZAR": {"R", 2, 100},
	"AED": {"AED", 2, 100},
	"SAR": {"SAR", 2, 100},
	"ILS": {"₪", 2, 100},
	"TRY": {"₺", 2, 100},
}

// FormatMoney memformat nilai float (sudah dalam unit utama) ke string mata uang.
// Gunakan ini untuk nilai dari Meta Insights API (spend, CPC, CPM, dll).
func FormatMoney(amount float64, currency string) string {
	code := DefaultCurrency(currency)
	meta, ok := currencyMap[code]
	if !ok {
		return fmt.Sprintf("%s %.2f", code, amount)
	}
	if meta.Decimals == 0 {
		return meta.Symbol + " " + formatThousands(int64(math.Round(amount)))
	}
	format := fmt.Sprintf("%s %%.%df", meta.Symbol, meta.Decimals)
	return fmt.Sprintf(format, amount)
}

// FormatBudget memformat nilai budget dari unit terkecil Meta API ke tampilan.
// Meta menyimpan budget dalam unit terkecil: USD dalam sen, IDR dalam rupiah.
func FormatBudget(smallestUnit int64, currency string) string {
	code := DefaultCurrency(currency)
	meta, ok := currencyMap[code]
	if !ok {
		return fmt.Sprintf("%s %.2f", code, float64(smallestUnit)/100.0)
	}
	amount := float64(smallestUnit) / meta.Divisor
	return FormatMoney(amount, currency)
}

// CurrencySymbol mengembalikan simbol mata uang.
func CurrencySymbol(currency string) string {
	if m, ok := currencyMap[DefaultCurrency(currency)]; ok {
		return m.Symbol
	}
	return currency
}

// CurrencyDecimals mengembalikan jumlah desimal untuk currency.
func CurrencyDecimals(currency string) int {
	if m, ok := currencyMap[DefaultCurrency(currency)]; ok {
		return m.Decimals
	}
	return 2
}

// ToSmallestUnit mengkonversi nilai dalam unit utama ke unit terkecil untuk Meta API.
// Contoh: IDR 100000 → 100000 (IDR tidak punya sub-unit)
//         USD 10.00 → 1000 (sen)
func ToSmallestUnit(amount float64, currency string) int64 {
	code := DefaultCurrency(currency)
	meta, ok := currencyMap[code]
	if !ok {
		return int64(math.Round(amount * 100))
	}
	return int64(math.Round(amount * meta.Divisor))
}

// DefaultCurrency mengembalikan IDR jika kosong, atau uppercase dari input.
func DefaultCurrency(currency string) string {
	if strings.TrimSpace(currency) == "" {
		return "IDR"
	}
	return strings.ToUpper(strings.TrimSpace(currency))
}

// formatThousands menambahkan separator ribuan menggunakan titik (format Indonesia).
func formatThousands(n int64) string {
	if n < 0 {
		return "-" + formatThousands(-n)
	}
	s := fmt.Sprintf("%d", n)
	if len(s) <= 3 {
		return s
	}
	result := make([]byte, 0, len(s)+len(s)/3)
	for i, ch := range s {
		if i > 0 && (len(s)-i)%3 == 0 {
			result = append(result, '.')
		}
		result = append(result, byte(ch))
	}
	return string(result)
}
