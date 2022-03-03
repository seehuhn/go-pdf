// seehuhn.de/go/pdf - a library for reading and writing PDF files
// Copyright (C) 2022  Jochen Voss <voss@seehuhn.de>
//
// This program is free software: you can redistribute it and/or modify
// it under the terms of the GNU General Public License as published by
// the Free Software Foundation, either version 3 of the License, or
// (at your option) any later version.
//
// This program is distributed in the hope that it will be useful,
// but WITHOUT ANY WARRANTY; without even the implied warranty of
// MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
// GNU General Public License for more details.
//
// You should have received a copy of the GNU General Public License
// along with this program.  If not, see <https://www.gnu.org/licenses/>.

package locale

import "fmt"

// Country represents a RFC 3066 country subtag,
// denoting the area to which a language variant relates.
type Country uint16

// String returns the two-letter ISO 3166-1 code for a country.
// https://en.wikipedia.org/wiki/ISO_3166-1_alpha-2#Officially_assigned_code_elements
func (c Country) String() string {
	country, ok := countries[c]
	if ok {
		return country.A2
	}
	return fmt.Sprintf("Country(%d)", c)
}

// Alpha3 returns the three-letter ISO 3166-1 code for a country.
// https://en.wikipedia.org/wiki/ISO_3166-1_alpha-3
func (c Country) Alpha3() string {
	return countries[c].A3
}

// Name returns the English short name for a country.
// https://en.wikipedia.org/wiki/ISO_3166-1#Officially_assigned_code_elements
func (c Country) Name() string {
	return countries[c].A3
}

// List of all ISO 3166-1 countries.
// https://en.wikipedia.org/wiki/ISO_3166-1#Officially_assigned_code_elements
const (
	CountryUndefined Country = 0

	CountryAFG Country = 4   // Afghanistan
	CountryALB Country = 8   // Albania
	CountryDZA Country = 12  // Algeria
	CountryAND Country = 20  // Andorra
	CountryAGO Country = 24  // Angola
	CountryATG Country = 28  // Antigua and Barbuda
	CountryAZE Country = 31  // Azerbaijan
	CountryARG Country = 32  // Argentina
	CountryAUS Country = 36  // Australia
	CountryAUT Country = 40  // Austria
	CountryBHS Country = 44  // Bahamas
	CountryBHR Country = 48  // Bahrain
	CountryBGD Country = 50  // Bangladesh
	CountryARM Country = 51  // Armenia
	CountryBRB Country = 52  // Barbados
	CountryBEL Country = 56  // Belgium
	CountryBTN Country = 64  // Bhutan
	CountryBOL Country = 68  // Bolivia
	CountryBIH Country = 70  // Bosnia and Herzegovina
	CountryBWA Country = 72  // Botswana
	CountryBRA Country = 76  // Brazil
	CountryBLZ Country = 84  // Belize
	CountrySLB Country = 90  // Solomon Islands
	CountryBRN Country = 96  // Brunei Darussalam
	CountryBGR Country = 100 // Bulgaria
	CountryMMR Country = 104 // Myanmar
	CountryBDI Country = 108 // Burundi
	CountryBLR Country = 112 // Belarus
	CountryKHM Country = 116 // Cambodia
	CountryCMR Country = 120 // Cameroon
	CountryCAN Country = 124 // Canada
	CountryCPV Country = 132 // Cabo Verde
	CountryCAF Country = 140 // Central African Republic
	CountryLKA Country = 144 // Sri Lanka
	CountryTCD Country = 148 // Chad
	CountryCHL Country = 152 // Chile
	CountryCHN Country = 156 // China
	CountryCOL Country = 170 // Colombia
	CountryCOM Country = 174 // Comoros
	CountryCOG Country = 178 // Republic of the Congo
	CountryCOD Country = 180 // Democratic Republic of the Congo
	CountryCRI Country = 188 // Costa Rica
	CountryHRV Country = 191 // Croatia
	CountryCUB Country = 192 // Cuba
	CountryCYP Country = 196 // Cyprus
	CountryCZE Country = 203 // Czech Republic
	CountryBEN Country = 204 // Benin
	CountryDNK Country = 208 // Denmark
	CountryDMA Country = 212 // Dominica
	CountryDOM Country = 214 // Dominican Republic
	CountryECU Country = 218 // Ecuador
	CountrySLV Country = 222 // El Salvador
	CountryGNQ Country = 226 // Equatorial Guinea
	CountryETH Country = 231 // Ethiopia
	CountryERI Country = 232 // Eritrea
	CountryEST Country = 233 // Estonia
	CountryFJI Country = 242 // Fiji
	CountryFIN Country = 246 // Finland
	CountryFRA Country = 250 // France
	CountryDJI Country = 262 // Djibouti
	CountryGAB Country = 266 // Gabon
	CountryGEO Country = 268 // Georgia (country)
	CountryGMB Country = 270 // Gambia
	CountryDEU Country = 276 // Germany
	CountryGHA Country = 288 // Ghana
	CountryKIR Country = 296 // Kiribati
	CountryGRC Country = 300 // Greece
	CountryGRD Country = 308 // Grenada
	CountryGTM Country = 320 // Guatemala
	CountryGIN Country = 324 // Guinea
	CountryGUY Country = 328 // Guyana
	CountryHTI Country = 332 // Haiti
	CountryVAT Country = 336 // Vatican City
	CountryHND Country = 340 // Honduras
	CountryHUN Country = 348 // Hungary
	CountryISL Country = 352 // Iceland
	CountryIND Country = 356 // India
	CountryIDN Country = 360 // Indonesia
	CountryIRN Country = 364 // Iran
	CountryIRQ Country = 368 // Iraq
	CountryIRL Country = 372 // Republic of Ireland
	CountryISR Country = 376 // Israel
	CountryITA Country = 380 // Italy
	CountryCIV Country = 384 // Ivory Coast
	CountryJAM Country = 388 // Jamaica
	CountryJPN Country = 392 // Japan
	CountryKAZ Country = 398 // Kazakhstan
	CountryJOR Country = 400 // Jordan
	CountryKEN Country = 404 // Kenya
	CountryPRK Country = 408 // North Korea
	CountryKOR Country = 410 // South Korea
	CountryKWT Country = 414 // Kuwait
	CountryKGZ Country = 417 // Kyrgyzstan
	CountryLAO Country = 418 // Laos
	CountryLBN Country = 422 // Lebanon
	CountryLSO Country = 426 // Lesotho
	CountryLVA Country = 428 // Latvia
	CountryLBR Country = 430 // Liberia
	CountryLBY Country = 434 // Libya
	CountryLIE Country = 438 // Liechtenstein
	CountryLTU Country = 440 // Lithuania
	CountryLUX Country = 442 // Luxembourg
	CountryMDG Country = 450 // Madagascar
	CountryMWI Country = 454 // Malawi
	CountryMYS Country = 458 // Malaysia
	CountryMDV Country = 462 // Maldives
	CountryMLI Country = 466 // Mali
	CountryMLT Country = 470 // Malta
	CountryMRT Country = 478 // Mauritania
	CountryMUS Country = 480 // Mauritius
	CountryMEX Country = 484 // Mexico
	CountryMCO Country = 492 // Monaco
	CountryMNG Country = 496 // Mongolia
	CountryMDA Country = 498 // Moldova
	CountryMNE Country = 499 // Montenegro
	CountryMAR Country = 504 // Morocco
	CountryMOZ Country = 508 // Mozambique
	CountryOMN Country = 512 // Oman
	CountryNAM Country = 516 // Namibia
	CountryNRU Country = 520 // Nauru
	CountryNPL Country = 524 // Nepal
	CountryNLD Country = 528 // Kingdom of the Netherlands
	CountryVUT Country = 548 // Vanuatu
	CountryNZL Country = 554 // New Zealand
	CountryNIC Country = 558 // Nicaragua
	CountryNER Country = 562 // Niger
	CountryNGA Country = 566 // Nigeria
	CountryNOR Country = 578 // Norway
	CountryFSM Country = 583 // Federated States of Micronesia
	CountryMHL Country = 584 // Marshall Islands
	CountryPLW Country = 585 // Palau
	CountryPAK Country = 586 // Pakistan
	CountryPAN Country = 591 // Panama
	CountryPNG Country = 598 // Papua New Guinea
	CountryPRY Country = 600 // Paraguay
	CountryPER Country = 604 // Peru
	CountryPHL Country = 608 // Philippines
	CountryPOL Country = 616 // Poland
	CountryPRT Country = 620 // Portugal
	CountryGNB Country = 624 // Guinea-Bissau
	CountryTLS Country = 626 // East Timor
	CountryQAT Country = 634 // Qatar
	CountryROU Country = 642 // Romania
	CountryRUS Country = 643 // Russia
	CountryRWA Country = 646 // Rwanda
	CountryKNA Country = 659 // Saint Kitts and Nevis
	CountryLCA Country = 662 // Saint Lucia
	CountryVCT Country = 670 // Saint Vincent and the Grenadines
	CountrySMR Country = 674 // San Marino
	CountrySTP Country = 678 // Sao Tome and Principe
	CountrySAU Country = 682 // Saudi Arabia
	CountrySEN Country = 686 // Senegal
	CountrySRB Country = 688 // Serbia
	CountrySYC Country = 690 // Seychelles
	CountrySLE Country = 694 // Sierra Leone
	CountrySGP Country = 702 // Singapore
	CountrySVK Country = 703 // Slovakia
	CountryVNM Country = 704 // Vietnam
	CountrySVN Country = 705 // Slovenia
	CountrySOM Country = 706 // Somalia
	CountryZAF Country = 710 // South Africa
	CountryZWE Country = 716 // Zimbabwe
	CountryESP Country = 724 // Spain
	CountrySSD Country = 728 // South Sudan
	CountrySDN Country = 729 // Sudan
	CountrySUR Country = 740 // Suriname
	CountrySWZ Country = 748 // Eswatini
	CountrySWE Country = 752 // Sweden
	CountryCHE Country = 756 // Switzerland
	CountrySYR Country = 760 // Syria
	CountryTJK Country = 762 // Tajikistan
	CountryTHA Country = 764 // Thailand
	CountryTGO Country = 768 // Togo
	CountryTON Country = 776 // Tonga
	CountryTTO Country = 780 // Trinidad and Tobago
	CountryARE Country = 784 // United Arab Emirates
	CountryTUN Country = 788 // Tunisia
	CountryTUR Country = 792 // Turkey
	CountryTKM Country = 795 // Turkmenistan
	CountryTUV Country = 798 // Tuvalu
	CountryUGA Country = 800 // Uganda
	CountryUKR Country = 804 // Ukraine
	CountryMKD Country = 807 // North Macedonia
	CountryEGY Country = 818 // Egypt
	CountryGBR Country = 826 // United Kingdom
	CountryTZA Country = 834 // Tanzania
	CountryUSA Country = 840 // United States
	CountryBFA Country = 854 // Burkina Faso
	CountryURY Country = 858 // Uruguay
	CountryUZB Country = 860 // Uzbekistan
	CountryVEN Country = 862 // Venezuela
	CountryWSM Country = 882 // Samoa
	CountryYEM Country = 887 // Yemen
	CountryZMB Country = 894 // Zambia
)

type countryCodes struct {
	A2 string
	A3 string
	N  string
}

var countries = map[Country]countryCodes{
	CountryAFG: {"AF", "AFG", "Afghanistan"},
	CountryALB: {"AL", "ALB", "Albania"},
	CountryDZA: {"DZ", "DZA", "Algeria"},
	CountryAND: {"AD", "AND", "Andorra"},
	CountryAGO: {"AO", "AGO", "Angola"},
	CountryATG: {"AG", "ATG", "Antigua and Barbuda"},
	CountryAZE: {"AZ", "AZE", "Azerbaijan"},
	CountryARG: {"AR", "ARG", "Argentina"},
	CountryAUS: {"AU", "AUS", "Australia"},
	CountryAUT: {"AT", "AUT", "Austria"},
	CountryBHS: {"BS", "BHS", "Bahamas"},
	CountryBHR: {"BH", "BHR", "Bahrain"},
	CountryBGD: {"BD", "BGD", "Bangladesh"},
	CountryARM: {"AM", "ARM", "Armenia"},
	CountryBRB: {"BB", "BRB", "Barbados"},
	CountryBEL: {"BE", "BEL", "Belgium"},
	CountryBTN: {"BT", "BTN", "Bhutan"},
	CountryBOL: {"BO", "BOL", "Bolivia"},
	CountryBIH: {"BA", "BIH", "Bosnia and Herzegovina"},
	CountryBWA: {"BW", "BWA", "Botswana"},
	CountryBRA: {"BR", "BRA", "Brazil"},
	CountryBLZ: {"BZ", "BLZ", "Belize"},
	CountrySLB: {"SB", "SLB", "Solomon Islands"},
	CountryBRN: {"BN", "BRN", "Brunei Darussalam"},
	CountryBGR: {"BG", "BGR", "Bulgaria"},
	CountryMMR: {"MM", "MMR", "Myanmar"},
	CountryBDI: {"BI", "BDI", "Burundi"},
	CountryBLR: {"BY", "BLR", "Belarus"},
	CountryKHM: {"KH", "KHM", "Cambodia"},
	CountryCMR: {"CM", "CMR", "Cameroon"},
	CountryCAN: {"CA", "CAN", "Canada"},
	CountryCPV: {"CV", "CPV", "Cabo Verde"},
	CountryCAF: {"CF", "CAF", "Central African Republic"},
	CountryLKA: {"LK", "LKA", "Sri Lanka"},
	CountryTCD: {"TD", "TCD", "Chad"},
	CountryCHL: {"CL", "CHL", "Chile"},
	CountryCHN: {"CN", "CHN", "China"},
	CountryCOL: {"CO", "COL", "Colombia"},
	CountryCOM: {"KM", "COM", "Comoros"},
	CountryCOG: {"CG", "COG", "Republic of the Congo"},
	CountryCOD: {"CD", "COD", "Democratic Republic of the Congo"},
	CountryCRI: {"CR", "CRI", "Costa Rica"},
	CountryHRV: {"HR", "HRV", "Croatia"},
	CountryCUB: {"CU", "CUB", "Cuba"},
	CountryCYP: {"CY", "CYP", "Cyprus"},
	CountryCZE: {"CZ", "CZE", "Czech Republic"},
	CountryBEN: {"BJ", "BEN", "Benin"},
	CountryDNK: {"DK", "DNK", "Denmark"},
	CountryDMA: {"DM", "DMA", "Dominica"},
	CountryDOM: {"DO", "DOM", "Dominican Republic"},
	CountryECU: {"EC", "ECU", "Ecuador"},
	CountrySLV: {"SV", "SLV", "El Salvador"},
	CountryGNQ: {"GQ", "GNQ", "Equatorial Guinea"},
	CountryETH: {"ET", "ETH", "Ethiopia"},
	CountryERI: {"ER", "ERI", "Eritrea"},
	CountryEST: {"EE", "EST", "Estonia"},
	CountryFJI: {"FJ", "FJI", "Fiji"},
	CountryFIN: {"FI", "FIN", "Finland"},
	CountryFRA: {"FR", "FRA", "France"},
	CountryDJI: {"DJ", "DJI", "Djibouti"},
	CountryGAB: {"GA", "GAB", "Gabon"},
	CountryGEO: {"GE", "GEO", "Georgia (country)"},
	CountryGMB: {"GM", "GMB", "Gambia"},
	CountryDEU: {"DE", "DEU", "Germany"},
	CountryGHA: {"GH", "GHA", "Ghana"},
	CountryKIR: {"KI", "KIR", "Kiribati"},
	CountryGRC: {"GR", "GRC", "Greece"},
	CountryGRD: {"GD", "GRD", "Grenada"},
	CountryGTM: {"GT", "GTM", "Guatemala"},
	CountryGIN: {"GN", "GIN", "Guinea"},
	CountryGUY: {"GY", "GUY", "Guyana"},
	CountryHTI: {"HT", "HTI", "Haiti"},
	CountryVAT: {"VA", "VAT", "Vatican City"},
	CountryHND: {"HN", "HND", "Honduras"},
	CountryHUN: {"HU", "HUN", "Hungary"},
	CountryISL: {"IS", "ISL", "Iceland"},
	CountryIND: {"IN", "IND", "India"},
	CountryIDN: {"ID", "IDN", "Indonesia"},
	CountryIRN: {"IR", "IRN", "Iran"},
	CountryIRQ: {"IQ", "IRQ", "Iraq"},
	CountryIRL: {"IE", "IRL", "Republic of Ireland"},
	CountryISR: {"IL", "ISR", "Israel"},
	CountryITA: {"IT", "ITA", "Italy"},
	CountryCIV: {"CI", "CIV", "Ivory Coast"},
	CountryJAM: {"JM", "JAM", "Jamaica"},
	CountryJPN: {"JP", "JPN", "Japan"},
	CountryKAZ: {"KZ", "KAZ", "Kazakhstan"},
	CountryJOR: {"JO", "JOR", "Jordan"},
	CountryKEN: {"KE", "KEN", "Kenya"},
	CountryPRK: {"KP", "PRK", "North Korea"},
	CountryKOR: {"KR", "KOR", "South Korea"},
	CountryKWT: {"KW", "KWT", "Kuwait"},
	CountryKGZ: {"KG", "KGZ", "Kyrgyzstan"},
	CountryLAO: {"LA", "LAO", "Laos"},
	CountryLBN: {"LB", "LBN", "Lebanon"},
	CountryLSO: {"LS", "LSO", "Lesotho"},
	CountryLVA: {"LV", "LVA", "Latvia"},
	CountryLBR: {"LR", "LBR", "Liberia"},
	CountryLBY: {"LY", "LBY", "Libya"},
	CountryLIE: {"LI", "LIE", "Liechtenstein"},
	CountryLTU: {"LT", "LTU", "Lithuania"},
	CountryLUX: {"LU", "LUX", "Luxembourg"},
	CountryMDG: {"MG", "MDG", "Madagascar"},
	CountryMWI: {"MW", "MWI", "Malawi"},
	CountryMYS: {"MY", "MYS", "Malaysia"},
	CountryMDV: {"MV", "MDV", "Maldives"},
	CountryMLI: {"ML", "MLI", "Mali"},
	CountryMLT: {"MT", "MLT", "Malta"},
	CountryMRT: {"MR", "MRT", "Mauritania"},
	CountryMUS: {"MU", "MUS", "Mauritius"},
	CountryMEX: {"MX", "MEX", "Mexico"},
	CountryMCO: {"MC", "MCO", "Monaco"},
	CountryMNG: {"MN", "MNG", "Mongolia"},
	CountryMDA: {"MD", "MDA", "Moldova"},
	CountryMNE: {"ME", "MNE", "Montenegro"},
	CountryMAR: {"MA", "MAR", "Morocco"},
	CountryMOZ: {"MZ", "MOZ", "Mozambique"},
	CountryOMN: {"OM", "OMN", "Oman"},
	CountryNAM: {"NA", "NAM", "Namibia"},
	CountryNRU: {"NR", "NRU", "Nauru"},
	CountryNPL: {"NP", "NPL", "Nepal"},
	CountryNLD: {"NL", "NLD", "Kingdom of the Netherlands"},
	CountryVUT: {"VU", "VUT", "Vanuatu"},
	CountryNZL: {"NZ", "NZL", "New Zealand"},
	CountryNIC: {"NI", "NIC", "Nicaragua"},
	CountryNER: {"NE", "NER", "Niger"},
	CountryNGA: {"NG", "NGA", "Nigeria"},
	CountryNOR: {"NO", "NOR", "Norway"},
	CountryFSM: {"FM", "FSM", "Federated States of Micronesia"},
	CountryMHL: {"MH", "MHL", "Marshall Islands"},
	CountryPLW: {"PW", "PLW", "Palau"},
	CountryPAK: {"PK", "PAK", "Pakistan"},
	CountryPAN: {"PA", "PAN", "Panama"},
	CountryPNG: {"PG", "PNG", "Papua New Guinea"},
	CountryPRY: {"PY", "PRY", "Paraguay"},
	CountryPER: {"PE", "PER", "Peru"},
	CountryPHL: {"PH", "PHL", "Philippines"},
	CountryPOL: {"PL", "POL", "Poland"},
	CountryPRT: {"PT", "PRT", "Portugal"},
	CountryGNB: {"GW", "GNB", "Guinea-Bissau"},
	CountryTLS: {"TL", "TLS", "East Timor"},
	CountryQAT: {"QA", "QAT", "Qatar"},
	CountryROU: {"RO", "ROU", "Romania"},
	CountryRUS: {"RU", "RUS", "Russia"},
	CountryRWA: {"RW", "RWA", "Rwanda"},
	CountryKNA: {"KN", "KNA", "Saint Kitts and Nevis"},
	CountryLCA: {"LC", "LCA", "Saint Lucia"},
	CountryVCT: {"VC", "VCT", "Saint Vincent and the Grenadines"},
	CountrySMR: {"SM", "SMR", "San Marino"},
	CountrySTP: {"ST", "STP", "Sao Tome and Principe"},
	CountrySAU: {"SA", "SAU", "Saudi Arabia"},
	CountrySEN: {"SN", "SEN", "Senegal"},
	CountrySRB: {"RS", "SRB", "Serbia"},
	CountrySYC: {"SC", "SYC", "Seychelles"},
	CountrySLE: {"SL", "SLE", "Sierra Leone"},
	CountrySGP: {"SG", "SGP", "Singapore"},
	CountrySVK: {"SK", "SVK", "Slovakia"},
	CountryVNM: {"VN", "VNM", "Vietnam"},
	CountrySVN: {"SI", "SVN", "Slovenia"},
	CountrySOM: {"SO", "SOM", "Somalia"},
	CountryZAF: {"ZA", "ZAF", "South Africa"},
	CountryZWE: {"ZW", "ZWE", "Zimbabwe"},
	CountryESP: {"ES", "ESP", "Spain"},
	CountrySSD: {"SS", "SSD", "South Sudan"},
	CountrySDN: {"SD", "SDN", "Sudan"},
	CountrySUR: {"SR", "SUR", "Suriname"},
	CountrySWZ: {"SZ", "SWZ", "Eswatini"},
	CountrySWE: {"SE", "SWE", "Sweden"},
	CountryCHE: {"CH", "CHE", "Switzerland"},
	CountrySYR: {"SY", "SYR", "Syria"},
	CountryTJK: {"TJ", "TJK", "Tajikistan"},
	CountryTHA: {"TH", "THA", "Thailand"},
	CountryTGO: {"TG", "TGO", "Togo"},
	CountryTON: {"TO", "TON", "Tonga"},
	CountryTTO: {"TT", "TTO", "Trinidad and Tobago"},
	CountryARE: {"AE", "ARE", "United Arab Emirates"},
	CountryTUN: {"TN", "TUN", "Tunisia"},
	CountryTUR: {"TR", "TUR", "Turkey"},
	CountryTKM: {"TM", "TKM", "Turkmenistan"},
	CountryTUV: {"TV", "TUV", "Tuvalu"},
	CountryUGA: {"UG", "UGA", "Uganda"},
	CountryUKR: {"UA", "UKR", "Ukraine"},
	CountryMKD: {"MK", "MKD", "North Macedonia"},
	CountryEGY: {"EG", "EGY", "Egypt"},
	CountryGBR: {"GB", "GBR", "United Kingdom"},
	CountryTZA: {"TZ", "TZA", "Tanzania"},
	CountryUSA: {"US", "USA", "United States"},
	CountryBFA: {"BF", "BFA", "Burkina Faso"},
	CountryURY: {"UY", "URY", "Uruguay"},
	CountryUZB: {"UZ", "UZB", "Uzbekistan"},
	CountryVEN: {"VE", "VEN", "Venezuela"},
	CountryWSM: {"WS", "WSM", "Samoa"},
	CountryYEM: {"YE", "YEM", "Yemen"},
	CountryZMB: {"ZM", "ZMB", "Zambia"},
}
