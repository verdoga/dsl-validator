package dslversions

import (
	"fmt"
	"strings"
)

// Supported возвращает новую копию каталога в числовом порядке; сейчас [V1_2].
// Изменение возвращённого среза не меняет поддержку версий приложением.
func Supported() []Version {
	return []Version{V1_2}
}

// RequireSupported проверяет запись через Parse и членство в Supported.
// Пустая, некорректная либо отсутствующая в каталоге версия даёт error
// «версия DSL <значение> не поддерживается; поддерживаются: <список>»;
// пустая версия отображается как «не задана».
// Не подставляет 1.2, не расширяет каталог и не меняет входные данные.
func RequireSupported(version Version) error {
	parsed, err := Parse(string(version))
	supported := Supported()
	if err == nil {
		for _, candidate := range supported {
			if parsed == candidate {
				return nil
			}
		}
	}

	displayVersion := string(version)
	if displayVersion == "" {
		displayVersion = "не задана"
	}
	names := make([]string, len(supported))
	for i, candidate := range supported {
		names[i] = string(candidate)
	}
	return fmt.Errorf("версия DSL %s не поддерживается; поддерживаются: %s", displayVersion, strings.Join(names, ", "))
}
