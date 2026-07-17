package main

import (
	"bytes"
	"compress/gzip"
	"io"
	"net/http"
	"regexp"
	"strings"
)

type ReplacementRule struct {
	Pattern *regexp.Regexp
	Replace string
}

func buildRule(key string, fromVal string, toVal string) ReplacementRule {
	escapedKey := regexp.QuoteMeta(key)
	pattern := `"` + escapedKey + `"\s*:\s*` + regexp.QuoteMeta(fromVal)
	return ReplacementRule{
		Pattern: regexp.MustCompile(pattern),
		Replace: `"` + key + `": ` + toVal,
	}
}

func buildURLRule(key string) ReplacementRule {
	escapedKey := regexp.QuoteMeta(key)
	pattern := `"` + escapedKey + `"\s*:\s*"[^"]*"`
	return ReplacementRule{
		Pattern: regexp.MustCompile(pattern),
		Replace: `"` + key + `": ""`,
	}
}

var replacementRules []ReplacementRule

func init() {
	replacementRules = []ReplacementRule{
		buildRule(decodeStr(vgEnabledObf), "true", "false"),
		buildRule(decodeStr(vgBgInstallObf), "true", "false"),
		buildRule(decodeStr(lolVgEnabledObf), "true", "false"),
		buildRule(decodeStr(lolVgEmbeddedObf), "true", "false"),
		buildRule(decodeStr(ksVgAttestObf), "true", "false"),
		buildRule(decodeStr(ksVgLaunchDisabledObf), "false", "true"),
		buildRule(decodeStr(ksRestartDisabledObf), "false", "true"),
		buildRule(decodeStr(ksVgLaunchObf), "true", "false"),
		buildURLRule(decodeStr(lolVgUrlObf)),
	}
}

func ModifyPayload(body []byte, headers http.Header) ([]byte, bool) {
	contentEncoding := headers.Get("Content-Encoding")

	var plainBody []byte
	var wasCompressed bool

	if strings.EqualFold(contentEncoding, "gzip") {
		reader, err := gzip.NewReader(bytes.NewReader(body))
		if err != nil {
			return body, false
		}
		defer reader.Close()

		plainBody, err = io.ReadAll(reader)
		if err != nil {
			return body, false
		}
		wasCompressed = true
	} else {
		plainBody = body
	}

	text := string(plainBody)
	modified := false

	for _, rule := range replacementRules {
		if rule.Pattern.MatchString(text) {
			text = rule.Pattern.ReplaceAllString(text, rule.Replace)
			modified = true
		}
	}

	if modified || wasCompressed {
		return []byte(text), modified
	}

	return body, false
}
