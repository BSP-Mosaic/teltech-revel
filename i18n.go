package revel

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/robfig/config"
	"github.com/BSP-Mosaic/teltech-glog"
)

const (
	CurrentLocaleRenderArg = "currentLocale" // The key for the current locale render arg value

	messageFilesDirectory = "messages"
	messageFilePattern    = `^\w+.[a-zA-Z]{2}$`
	unknownValueFormat    = "??? %s ???"
	defaultLanguageOption = "i18n.default_language"
	localeCookieConfigKey = "i18n.cookie"
)

var (
	// All currently loaded message configs.
	messages map[string]*config.Config
)

// Return all currently loaded message languages.
func MessageLanguages() []string {
	languages := make([]string, len(messages))
	i := 0
	for language, _ := range messages {
		languages[i] = language
		i++
	}
	return languages
}

// Perform a message look-up for the given locale and message using the given arguments.
//
// When either an unknown locale or message is detected, a specially formatted string is returned.
func Message(locale, message string, args ...interface{}) string {
	language, region := parseLocale(locale)
	glog.V(1).Infof("Resolving message '%s' for language '%s' and region '%s'", message, language, region)

	var value string
	var err error
	messageConfig, knownLanguage := messages[language]
	if knownLanguage {
		// This works because unlike the goconfig documentation suggests it will actually
		// try to resolve message in DEFAULT if it did not find it in the given section.
		value, err = messageConfig.String(region, message)
		if err != nil {
			glog.V(1).Infof("Unknown message '%s' for locale '%s', trying default language", message, locale)
			// Continue to try default language
		}
	} else {
		glog.V(1).Infof("Unsupported language for locale '%s' and message '%s', trying default language", locale, message)
	}

	if value == "" {
		if defaultLanguage, found := Config.String(defaultLanguageOption); found {
			glog.V(1).Infof("Using default language '%s'", defaultLanguage)

			messageConfig, knownLanguage = messages[defaultLanguage]
			if !knownLanguage {
				glog.Warningf("Unsupported default language for locale '%s' and message '%s'", defaultLanguage, message)
				return fmt.Sprintf(unknownValueFormat, message)
			}

			value, err = messageConfig.String(region, message)
			if err != nil {
				glog.Warningf("Unknown message '%s' for default locale '%s'", message, locale)
				return fmt.Sprintf(unknownValueFormat, message)
			}
		} else {
			glog.Warningf("Unable to find default language option (%s); messages for unsupported locales will never be translated", defaultLanguageOption)
			return fmt.Sprintf(unknownValueFormat, message)
		}
	}

	if len(args) > 0 {
		glog.V(1).Infof("Arguments detected, formatting '%s' with %v", value, args)
		value = fmt.Sprintf(value, args...)
	}

	return value
}

func parseLocale(locale string) (language, region string) {
	if strings.Contains(locale, "-") {
		languageAndRegion := strings.Split(locale, "-")
		return languageAndRegion[0], languageAndRegion[1]
	}

	return locale, ""
}

// Recursively read and cache all available messages from all message files on the given path.
func loadMessages(path string) {
	messages = make(map[string]*config.Config)

	if error := filepath.Walk(path, loadMessageFile); error != nil && !os.IsNotExist(error) {
		glog.Errorln("Error reading messages files:", error)
	}
}

// Load a single message file
func loadMessageFile(path string, info os.FileInfo, osError error) error {
	if osError != nil {
		return osError
	}
	if info.IsDir() {
		return nil
	}

	if matched, _ := regexp.MatchString(messageFilePattern, info.Name()); matched {
		if config, error := parseMessagesFile(path); error != nil {
			return error
		} else {
			locale := parseLocaleFromFileName(info.Name())

			// If we have already parsed a message file for this locale, merge both
			if _, exists := messages[locale]; exists {
				messages[locale].Merge(config)
				glog.V(1).Infof("Successfully merged messages for locale '%s'", locale)
			} else {
				messages[locale] = config
			}

			glog.V(1).Infoln("Successfully loaded messages from file", info.Name())
		}
	} else {
		glog.V(1).Infof("Ignoring file %s because it did not have a valid extension", info.Name())
	}

	return nil
}

func parseMessagesFile(path string) (messageConfig *config.Config, error error) {
	messageConfig, error = config.ReadDefault(path)
	return
}

func parseLocaleFromFileName(file string) string {
	extension := filepath.Ext(file)[1:]
	return strings.ToLower(extension)
}

func init() {
	OnAppStart(func() {
		loadMessages(filepath.Join(BasePath, messageFilesDirectory))
	})
}

func I18nFilter(c *Controller, fc []Filter) {
	if foundCookie, cookieValue := hasLocaleCookie(c.Request); foundCookie {
		glog.V(1).Infof("Found locale cookie value: %s", cookieValue)
		setCurrentLocaleControllerArguments(c, cookieValue)
	} else if foundHeader, headerValue := hasAcceptLanguageHeader(c.Request); foundHeader {
		glog.V(1).Infof("Found Accept-Language header value: %s", headerValue)
		setCurrentLocaleControllerArguments(c, headerValue)
	} else {
		glog.V(1).Info("Unable to find locale in cookie or header, using empty string")
		setCurrentLocaleControllerArguments(c, "")
	}
	fc[0](c, fc[1:])
}

// Set the current locale controller argument (CurrentLocaleControllerArg) with the given locale.
func setCurrentLocaleControllerArguments(c *Controller, locale string) {
	c.Request.Locale = locale
	c.RenderArgs[CurrentLocaleRenderArg] = locale
}

// Determine whether the given request has valid Accept-Language value.
//
// Assumes that the accept languages stored in the request are sorted according to quality, with top
// quality first in the slice.
func hasAcceptLanguageHeader(request *Request) (bool, string) {
	if request.AcceptLanguages != nil && len(request.AcceptLanguages) > 0 {
		return true, request.AcceptLanguages[0].Language
	}

	return false, ""
}

// Determine whether the given request has a valid language cookie value.
func hasLocaleCookie(request *Request) (bool, string) {
	if request != nil && request.Cookies() != nil {
		name := Config.StringDefault(localeCookieConfigKey, CookiePrefix+"_LANG")
		if cookie, error := request.Cookie(name); error == nil {
			return true, cookie.Value
		} else {
			glog.V(1).Infof("Unable to read locale cookie with name '%s': %s", name, error.Error())
		}
	}

	return false, ""
}
