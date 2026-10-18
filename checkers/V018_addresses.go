package checkers

import (
	"net"
	"net/mail"
	"net/url"
	"strings"
	"unicode"
)

// v018Addresses отмечает распознанные http(s)-адреса и почтовые адреса в одном
// видимом участке текста. Маска не включает конечную пунктуацию предложения;
// соседние слова и границы других элементов или ответов не присоединяются.
func v018Addresses(text []rune) []bool {
	excluded := make([]bool, len(text))
	for start := 0; start < len(text); {
		if !unicode.IsLetter(text[start]) && !unicode.IsDigit(text[start]) && text[start] != '_' ||
			start > 0 && v018AddressWord(text[start-1]) {
			start++
			continue
		}
		urlStart := start+7 <= len(text) && strings.EqualFold(string(text[start:start+7]), "http://") ||
			start+8 <= len(text) && strings.EqualFold(string(text[start:start+8]), "https://")
		end := start
		for end < len(text) && !unicode.IsSpace(text[end]) && !strings.ContainsRune("\"'<>[](){}|", text[end]) &&
			(urlStart || v018MailRune(text[end])) {
			end++
		}
		next := end
		for end > start && strings.ContainsRune(".,;:!?", text[end-1]) {
			end--
		}
		if !v018Address(string(text[start:end])) {
			// Каждый кандидат просматривается один раз; пунктуация между
			// адресами не вызывает повторного обхода всего остатка строки.
			start = next
			continue
		}
		for i := start; i < end; i++ {
			excluded[i] = true
		}
		start = end
	}
	return excluded
}

// v018MailRune возвращает true для символов непроцитированного почтового
// адреса. Двоеточие и разделители списка завершают кандидата.
func v018MailRune(char rune) bool {
	return unicode.IsLetter(char) || unicode.IsDigit(char) || unicode.IsMark(char) ||
		strings.ContainsRune(".!#$%&*+-/=?^_`~@", char)
}

// v018AddressWord возвращает true для символов, внутри которых новый адрес
// начинаться не может. Это не позволяет исключить суффикс ошибочного токена.
func v018AddressWord(char rune) bool {
	return unicode.IsLetter(char) || unicode.IsDigit(char) || unicode.IsMark(char) || strings.ContainsRune("._@/+-", char)
}

// v018Address возвращает true для URL с явной схемой http(s) и непустым
// допустимым хостом либо отдельного почтового адреса без имени отправителя.
// Сокращения, произвольные слова с точкой и фрагменты путей дают false.
func v018Address(candidate string) bool {
	lower := strings.ToLower(candidate)
	if strings.HasPrefix(lower, "https://") || strings.HasPrefix(lower, "http://") {
		parsed, err := url.Parse(candidate)
		return err == nil && v018AddressHost(parsed.Hostname())
	}
	if strings.Count(candidate, "@") != 1 {
		return false
	}
	parsed, err := mail.ParseAddress(candidate)
	if err != nil || parsed.Name != "" || parsed.Address != candidate {
		return false
	}
	return v018AddressHost(candidate[strings.LastIndexByte(candidate, '@')+1:])
}

// v018AddressHost возвращает true для IP-адреса или непустых доменных меток
// из букв, цифр и внутренних дефисов. Пунктуация текста не становится хостом.
func v018AddressHost(host string) bool {
	if net.ParseIP(host) != nil {
		return true
	}
	for _, label := range strings.Split(host, ".") {
		if label == "" || strings.HasPrefix(label, "-") || strings.HasSuffix(label, "-") {
			return false
		}
		for _, char := range label {
			if !unicode.IsLetter(char) && !unicode.IsDigit(char) && char != '-' {
				return false
			}
		}
	}
	return true
}
