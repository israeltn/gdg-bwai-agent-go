// Package farming provides eino-compatible tools for a Nigerian farming assistant.
// Each tool is built using eino's InferTool helper, which automatically derives
// the JSON schema for the LLM from the Go input struct's json/jsonschema tags.
package farming

import (
	"context"
	"fmt"
	"strings"

	"github.com/cloudwego/eino/components/tool"
	"github.com/cloudwego/eino/components/tool/utils"
)

// ─── Crop Price Tool ──────────────────────────────────────────────────────────

// CropPriceInput is the argument struct for the crop price tool.
// eino reads the json tags for parameter names and jsonschema_description for docs.
type CropPriceInput struct {
	Crop     string `json:"crop"     jsonschema:"required" jsonschema_description:"The crop name, e.g. yam, maize, tomato, rice, cassava, pepper"`
	Location string `json:"location"                       jsonschema_description:"Nigerian city, e.g. Lokoja, Abuja, Lagos, Kano. Defaults to Lokoja."`
}

var cropPrices = map[string]map[string]int{
	"yam":     {"lokoja": 850, "abuja": 920, "lagos": 1100, "kano": 750},
	"maize":   {"lokoja": 350, "abuja": 400, "lagos": 450, "kano": 300},
	"tomato":  {"lokoja": 600, "abuja": 700, "lagos": 800, "kano": 500},
	"rice":    {"lokoja": 1200, "abuja": 1350, "lagos": 1500, "kano": 1100},
	"cassava": {"lokoja": 180, "abuja": 220, "lagos": 280, "kano": 160},
	"pepper":  {"lokoja": 1500, "abuja": 1700, "lagos": 2000, "kano": 1300},
}

func cropPriceFn(_ context.Context, input CropPriceInput) (string, error) {
	crop := strings.ToLower(strings.TrimSpace(input.Crop))
	loc := strings.ToLower(strings.TrimSpace(input.Location))
	if loc == "" {
		loc = "lokoja"
	}

	cityPrices, ok := cropPrices[crop]
	if !ok {
		return fmt.Sprintf("No price data for %q. Known crops: yam, maize, tomato, rice, cassava, pepper.", crop), nil
	}

	price, ok := cityPrices[loc]
	if !ok {
		// fallback to Lokoja
		price = cityPrices["lokoja"]
		return fmt.Sprintf("Current price of %s in %s: ₦%d/kg (Lokoja price used as fallback).", crop, loc, price), nil
	}
	return fmt.Sprintf("Current market price of %s in %s: ₦%d per kg.", crop, strings.Title(loc), price), nil
}

// NewCropPriceTool creates an eino InvokableTool for crop prices.
func NewCropPriceTool() (tool.InvokableTool, error) {
	return utils.InferTool(
		"get_crop_price",
		"Get the current market price for a crop in a Nigerian city.",
		cropPriceFn,
	)
}

// ─── Weather Tool ─────────────────────────────────────────────────────────────

type WeatherInput struct {
	City string `json:"city" jsonschema:"required" jsonschema_description:"Nigerian city name, e.g. Lokoja, Abuja, Lagos, Kano, Ibadan"`
}

type weatherData struct {
	condition string
	tempC     int
	humidity  int
	forecast  string
}

var weatherDB = map[string]weatherData{
	"lokoja":        {"Partly cloudy", 34, 65, "Rain expected in 2 days — harvest soon if crops are ready"},
	"abuja":         {"Clear skies", 31, 55, "Dry for the next 3 days — good time for field work"},
	"lagos":         {"Overcast, light rain", 28, 80, "Heavy rain forecast tomorrow — delay any harvesting"},
	"kano":          {"Hot and dry", 40, 20, "No rain expected this week — irrigate if possible"},
	"ibadan":        {"Sunny", 33, 60, "Scattered showers in 3 days — normal farming conditions"},
	"port harcourt": {"Humid, cloudy", 27, 85, "Rain likely today and tomorrow — avoid outdoor drying"},
	"enugu":         {"Mild, partly cloudy", 29, 70, "Good conditions for planting this week"},
	"kaduna":        {"Clear", 35, 45, "Dry and warm — ideal for harvesting dry crops"},
	"jos":           {"Cool and cloudy", 24, 72, "Light rain possible — monitor crops for moisture"},
	"maiduguri":     {"Extremely hot", 43, 15, "Severe heat — early morning farming only"},
}

func weatherFn(_ context.Context, input WeatherInput) (string, error) {
	city := strings.ToLower(strings.TrimSpace(input.City))
	w, ok := weatherDB[city]
	if !ok {
		return fmt.Sprintf("No weather data for %q. Try: Lokoja, Abuja, Lagos, Kano, Ibadan, Enugu, Kaduna, Jos.", input.City), nil
	}
	return fmt.Sprintf(
		"Weather in %s: %s, %d°C, humidity %d%%.\nFarming forecast: %s.",
		strings.Title(city), w.condition, w.tempC, w.humidity, w.forecast,
	), nil
}

// NewWeatherTool creates an eino InvokableTool for weather data.
func NewWeatherTool() (tool.InvokableTool, error) {
	return utils.InferTool(
		"check_weather",
		"Get current weather and farming forecast for a Nigerian city.",
		weatherFn,
	)
}

// ─── Currency Tool ────────────────────────────────────────────────────────────

type CurrencyInput struct {
	Amount float64 `json:"amount"        jsonschema:"required" jsonschema_description:"The numeric amount to convert"`
	From   string  `json:"from_currency" jsonschema:"required" jsonschema_description:"Source currency code: NGN or USD"`
	To     string  `json:"to_currency"   jsonschema:"required" jsonschema_description:"Target currency code: NGN or USD"`
}

const usdToNGN = 1580.0

func currencyFn(_ context.Context, input CurrencyInput) (string, error) {
	from := strings.ToUpper(strings.TrimSpace(input.From))
	to := strings.ToUpper(strings.TrimSpace(input.To))

	switch {
	case from == "NGN" && to == "USD":
		result := input.Amount / usdToNGN
		return fmt.Sprintf("₦%.2f = $%.2f USD (rate: ₦%.0f per $1)", input.Amount, result, usdToNGN), nil
	case from == "USD" && to == "NGN":
		result := input.Amount * usdToNGN
		return fmt.Sprintf("$%.2f USD = ₦%.2f (rate: ₦%.0f per $1)", input.Amount, result, usdToNGN), nil
	default:
		return fmt.Sprintf("Unsupported conversion: %s to %s. Supported: NGN <-> USD", from, to), nil
	}
}

// NewCurrencyTool creates an eino InvokableTool for currency conversion.
func NewCurrencyTool() (tool.InvokableTool, error) {
	return utils.InferTool(
		"convert_currency",
		"Convert an amount between NGN (Nigerian Naira) and USD.",
		currencyFn,
	)
}

// ─── Farming Tip Tool ─────────────────────────────────────────────────────────

type FarmingTipInput struct {
	Crop string `json:"crop" jsonschema:"required" jsonschema_description:"Crop name: yam, maize, tomato, cassava, rice, or pepper"`
}

var farmingTips = map[string]string{
	"yam": "Yam Tips: Plant seed yams at start of rainy season (April-May). Space 1m x 1m, stake vines at 30cm. Apply NPK 15-15-15 at 6 and 10 weeks. Harvest when leaves turn yellow (7-9 months). Yields 15-25 tonnes/hectare.",
	"maize": "Maize Tips: Plant at onset of rain (March-April south, May-June north). Use SUWAN-1-SR for disease resistance. Space 75cm x 25cm, 2 seeds/hole. Apply urea top-dressing 4 weeks after planting. Watch for fall armyworm.",
	"tomato": "Tomato Tips: Best planted in dry season (Oct-Feb) with irrigation to avoid blight. Use raised beds; transplant at 4-6 weeks. Stake plants; drip irrigation saves 40% water. Harvest when red; sell same day.",
	"cassava": "Cassava Tips: Plant stem cuttings (25-30cm) at an angle in mounds. Drought tolerant — grows in poor soils. TMS 30572 gives high yield. Harvest after 9-12 months. Process within 24 hours to prevent cyanide buildup.",
	"rice": "Rice Tips: FARO 44 and NERICA varieties work best in Nigeria. Flooded paddy fields give highest yield. Transplant 21-day-old seedlings at 20cm x 20cm. Apply NPK at transplanting + urea at tillering. Dry to 14% moisture for storage.",
	"pepper": "Pepper Tips: Start seeds in nursery; transplant after 6 weeks. Needs 25-30°C and moderate water. Space 60cm x 60cm; mulch to retain moisture. Red pepper commands higher price. Sun-dry for preservation; sells 3x fresh price.",
}

func farmingTipFn(_ context.Context, input FarmingTipInput) (string, error) {
	crop := strings.ToLower(strings.TrimSpace(input.Crop))
	tip, ok := farmingTips[crop]
	if !ok {
		return fmt.Sprintf("No tips for %q. Available crops: yam, maize, tomato, cassava, rice, pepper.", crop), nil
	}
	return tip, nil
}

// NewFarmingTipTool creates an eino InvokableTool for farming advice.
func NewFarmingTipTool() (tool.InvokableTool, error) {
	return utils.InferTool(
		"get_farming_tip",
		"Get expert farming tips and advice for a specific Nigerian crop.",
		farmingTipFn,
	)
}

// ─── All Tools ────────────────────────────────────────────────────────────────

// AllTools returns all four farming tools as eino InvokableTools.
// Returns an error if any tool fails to build (which would indicate a bug).
func AllTools() ([]tool.InvokableTool, error) {
	builders := []func() (tool.InvokableTool, error){
		NewCropPriceTool,
		NewWeatherTool,
		NewCurrencyTool,
		NewFarmingTipTool,
	}
	tools := make([]tool.InvokableTool, 0, len(builders))
	for _, build := range builders {
		t, err := build()
		if err != nil {
			return nil, err
		}
		tools = append(tools, t)
	}
	return tools, nil
}
