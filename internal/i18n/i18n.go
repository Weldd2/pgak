package i18n

import (
	_ "embed"
	"encoding/json"
	"log"
)

//go:embed locales/en.json
var enJSON []byte

//go:embed locales/fr.json
var frJSON []byte

var translations map[string]map[string]string

var currentLang = "en" // default to English

func init() {
	translations = make(map[string]map[string]string)

	var enMap map[string]string
	if err := json.Unmarshal(enJSON, &enMap); err != nil {
		log.Printf("i18n: failed to load English translations: %v", err)
	} else {
		translations["en"] = enMap
	}

	var frMap map[string]string
	if err := json.Unmarshal(frJSON, &frMap); err != nil {
		log.Printf("i18n: failed to load French translations: %v", err)
	} else {
		translations["fr"] = frMap
	}
}

// SetLanguage sets the current language (en or fr)
func SetLanguage(lang string) {
	if lang == "en" || lang == "fr" {
		currentLang = lang
	}
}

// T returns the translation for the given key in the current language.
// Falls back to English if the key is not found.
func T(key string) string {
	if trans, ok := translations[currentLang][key]; ok {
		return trans
	}
	// fallback to English
	if trans, ok := translations["en"][key]; ok {
		return trans
	}
	// fallback to the key itself if not found
	return key
}

// CurrentLanguage returns the current language code
func CurrentLanguage() string {
	return currentLang
}
