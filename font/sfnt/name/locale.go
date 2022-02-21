package name

import "seehuhn.de/go/pdf/locale"

type loc struct {
	lang    locale.Language
	country locale.Country
}

// Selected Macintosh language codes
// https://docs.microsoft.com/en-us/typography/opentype/spec/name#macintosh-language-ids
var appleCodes = map[uint16]loc{
	0:  {lang: locale.LangEnglish},
	1:  {lang: locale.LangFrench},
	2:  {lang: locale.LangGerman},
	3:  {lang: locale.LangItalian},
	4:  {lang: locale.LangDutch},
	6:  {lang: locale.LangSpanish},
	12: {lang: locale.LangArabic},
	14: {lang: locale.LangGreek},
	21: {lang: locale.LangHindi},
	32: {lang: locale.LangRussian},
	37: {lang: locale.LangRomanian},
	67: {lang: locale.LangBengali},
}

// Selected Windows language codes
// https://docs.microsoft.com/en-us/typography/opentype/spec/name#windows-language-ids
var microsoftCodes = map[uint16]loc{
	0x0401: {lang: locale.LangArabic, country: locale.CountrySAU},
	0x0407: {lang: locale.LangGerman, country: locale.CountryDEU},
	0x0408: {lang: locale.LangGreek, country: locale.CountryGRC},
	0x0409: {lang: locale.LangEnglish, country: locale.CountryUSA},
	0x040A: {lang: locale.LangSpanish, country: locale.CountryESP}, // traditional sort
	0x040C: {lang: locale.LangFrench, country: locale.CountryFRA},
	0x0410: {lang: locale.LangItalian, country: locale.CountryITA},
	0x0413: {lang: locale.LangDutch, country: locale.CountryNLD},
	0x0418: {lang: locale.LangRomanian, country: locale.CountryROU},
	0x0419: {lang: locale.LangRussian, country: locale.CountryRUS},
	0x0439: {lang: locale.LangHindi, country: locale.CountryIND},
	0x0445: {lang: locale.LangBengali, country: locale.CountryIND},
	0x0804: {lang: locale.LangChinese, country: locale.CountryCHN},
	0x0809: {lang: locale.LangEnglish, country: locale.CountryGBR},
	0x0845: {lang: locale.LangBengali, country: locale.CountryBGD},
	0x0C0A: {lang: locale.LangSpanish, country: locale.CountryESP}, // modern sort
}
