package utils

// Mirrors busTints in app/src/lib/utils/tints.ts. Routes absent here look the
// same in both themes.
var darkModeRouteColors = map[string]string{
	"01-04": "#fe453a",
	"03":    "#03a8e4",
	"03-05": "#0285ff",
	"04":    "#ff9500",
	"05":    "#a387ff",
	"06":    "#d15cff",
	"07":    "#e9b11a",
	"08":    "#619bff",
	"12":    "#04bcef",
	"15":    "#34d070",
	"22":    "#dca200",
	"27":    "#29c753",
	"G31":   "#28c3d8",
	"34":    "#ff649a",
	"35":    "#ff9500",
	"G35":   "#ff9500",
	"36":    "#00a2ff",
	"40":    "#2da9de",
	"47-48": "#d187ff",
	"47":    "#05d56d",
	"48":    "#05d56d",
	"WR":    "#05d56d",
	"PARA":  "#7bd1ff",
	"BRY":   "#2da9de",
}

// DarkModeRouteColor returns the empty string when a route has no dark variant.
func DarkModeRouteColor(shortName string) string {
	return darkModeRouteColors[shortName]
}
