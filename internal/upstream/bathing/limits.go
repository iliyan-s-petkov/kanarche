package bathing

// Limits are the Bathing Water Directive (2006/7/EC, Annex I) reference values
// in cfu/100 ml: the "excellent" and "good" bounds for each indicator.
type Limits struct {
	EColi       [2]int `json:"e_coli"`
	Enterococci [2]int `json:"enterococci"`
}

// LimitsByZone keys by the stored zone. Inland waters (lakes) have looser bounds.
var LimitsByZone = map[string]Limits{
	"coastal": {EColi: [2]int{250, 500}, Enterococci: [2]int{100, 200}},
	"lake":    {EColi: [2]int{500, 1000}, Enterococci: [2]int{200, 400}},
}
