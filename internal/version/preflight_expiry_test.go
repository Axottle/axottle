package version

import (
	"testing"
	"time"
)

// Срок жизни preflight-сборки.
//
// Такой бинарь — это, по сути, бессрочная бесплатная лицензия: dev-режим
// включён принудительно, доверенных ключей нет, и ничто из этого само не
// заканчивается. Для диагностического артефакта на один вечер это нормально;
// для файла, который переживёт эту задачу, — нет.
func TestPreflightExpiry(t *testing.T) {
	origFlag, origExpiry := Preflight, PreflightExpiry
	t.Cleanup(func() { Preflight, PreflightExpiry = origFlag, origExpiry })

	Preflight = "true"

	t.Run("без штампа — как раньше, бессрочно", func(t *testing.T) {
		PreflightExpiry = ""
		if !IsPreflight() {
			t.Fatal("сборка без штампа срока должна оставаться preflight")
		}
	})

	t.Run("до срока — работает", func(t *testing.T) {
		PreflightExpiry = time.Now().Add(24 * time.Hour).UTC().Format(time.RFC3339)
		if !IsPreflight() {
			t.Fatal("непросроченная сборка должна оставаться preflight")
		}
	})

	t.Run("после срока — обычная панель", func(t *testing.T) {
		PreflightExpiry = time.Now().Add(-time.Minute).UTC().Format(time.RFC3339)
		if IsPreflight() {
			t.Fatal("просроченная сборка обязана перестать обходить лицензирование")
		}
	})

	// Кривой штамп куда вероятнее правки бинаря, чем ошибки упаковщика, а цена
	// ошибки в другую сторону — обход лицензии, который не кончается никогда.
	t.Run("нечитаемый штамп считается просроченным", func(t *testing.T) {
		PreflightExpiry = "позавчера"
		if IsPreflight() {
			t.Fatal("нечитаемый штамп срока должен трактоваться как просроченный")
		}
	})

	t.Run("штамп без флага ничего не включает", func(t *testing.T) {
		Preflight = ""
		PreflightExpiry = time.Now().Add(24 * time.Hour).UTC().Format(time.RFC3339)
		if IsPreflight() {
			t.Fatal("один только штамп срока не должен делать сборку preflight")
		}
	})
}
