package config

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"

	"github.com/spf13/viper"
)

// expandEnvWithDefaults расширяет переменные окружения с поддержкой дефолтных значений
// Формат: ${VAR:-default}
func expandEnvWithDefaults(s string) string {
	// Регулярное выражение для поиска ${VAR:-default}
	re := regexp.MustCompile(`\$\{([^}:]+)(?::-([^}]*))?\}`)

	return re.ReplaceAllStringFunc(s, func(match string) string {
		// Извлекаем имя переменной и значение по умолчанию
		matches := re.FindStringSubmatch(match)
		if len(matches) < 2 {
			return match
		}

		varName := matches[1]
		defaultValue := ""
		if len(matches) > 2 {
			defaultValue = matches[2]
		}

		// Пытаемся получить значение из переменных окружения
		value := os.Getenv(varName)
		if value == "" {
			// Если переменная не установлена, используем значение по умолчанию
			return defaultValue
		}
		return value
	})
}

// InitConfig читает конфигурационный файл и возвращает экземпляр конфигурации
// Использует generic для работы с произвольным типом конфигурации
func InitConfig[C any](configFile string) (*C, error) {
	v := viper.New()
	ext := strings.TrimLeft(filepath.Ext(configFile), ".")

	v.SetConfigFile(configFile)
	v.SetConfigType(ext)
	err := v.ReadInConfig()
	if err != nil {
		return nil, fmt.Errorf("v.ReadInConfig: %w", err)
	}

	// Заменяем переменные окружения формата ${VAR:-default} на их значения
	for _, k := range v.AllKeys() {
		value := v.GetString(k)
		if value == "" {
			continue
		}
		// Используем кастомную функцию для поддержки дефолтных значений
		expanded := expandEnvWithDefaults(value)

		// Пытаемся определить тип значения и установить его правильно
		// Если значение выглядит как число или boolean, пытаемся распарсить
		if expanded == "true" || expanded == "false" {
			boolValue, _ := strconv.ParseBool(expanded)
			v.Set(k, boolValue)
		} else if intValue, err := strconv.Atoi(expanded); err == nil {
			v.Set(k, intValue)
		} else {
			v.Set(k, expanded)
		}
	}

	cfg := new(C)
	if err := v.Unmarshal(cfg); err != nil {
		return nil, fmt.Errorf("v.Unmarshal: %w", err)
	}

	return cfg, nil
}
