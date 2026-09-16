package Graphite

import "testing"

func TestApplication_DefaultLocaleIsEnglish(t *testing.T) {
	app := NewApplication()
	if got := app.Locale(); got != LocaleEnglish {
		t.Errorf("default Locale() = %q, want %q", got, LocaleEnglish)
	}
	if got := app.T(KeyOK); got != "OK" {
		t.Errorf("T(KeyOK) = %q, want %q", got, "OK")
	}
}

func TestApplication_SetLocaleSwitchesBuiltinTranslation(t *testing.T) {
	app := NewApplication()
	app.SetLocale(LocaleUkrainian)

	if got := app.Locale(); got != LocaleUkrainian {
		t.Errorf("Locale() = %q, want %q", got, LocaleUkrainian)
	}
	if got := app.T(KeyCancel); got != "Скасувати" {
		t.Errorf("T(KeyCancel) = %q, want %q", got, "Скасувати")
	}
}

// Every locale builtinCatalogs ships must define every Key* constant, so a
// caller who switches locales never silently loses a string that worked in
// another language.
func TestBuiltinCatalogs_EveryLocaleHasEveryKey(t *testing.T) {
	keys := []string{
		KeyYes, KeyNo, KeyOK, KeyCancel, KeyValueRequired, KeyOpen,
		KeyDirLabel, KeyFileNameLabel, KeyColumnName, KeyColumnDate,
		KeyColumnType, KeyColumnSize, KeyFileFolder, KeyFileGeneric,
		KeyFilterAll, KeyFilterGph, KeyFilterImage, KeyFilterVideo,
		KeyValuePrompt, KeyValueRangeError,
	}

	for locale, catalog := range builtinCatalogs() {
		for _, key := range keys {
			if _, ok := catalog[key]; !ok {
				t.Errorf("locale %q is missing translation for key %q", locale, key)
			}
		}
	}
}

// Regression: T must format the resolved translation with args, not just
// return the raw text, so a translated ValuePrompt/ValueRangeError still
// substitutes the caller's numbers.
func TestApplication_TFormatsArgs(t *testing.T) {
	app := NewApplication()
	if got, want := app.T(KeyValuePrompt, 0.0, 100.0), "Value (0-100):"; got != want {
		t.Errorf("T(KeyValuePrompt, 0, 100) = %q, want %q", got, want)
	}

	app.SetLocale(LocaleGerman)
	if got, want := app.T(KeyValueRangeError, 1.0, 9.0), "Geben Sie eine Zahl zwischen 1 und 9 ein."; got != want {
		t.Errorf("T(KeyValueRangeError) in German = %q, want %q", got, want)
	}
}

// A key with no entry anywhere (not in a custom catalog, not built-in for
// the current locale, not built-in for English either) degrades to the key
// itself rather than an empty string, so a missing translation is at least
// visible and debuggable instead of silently blank.
func TestApplication_TFallsBackToKeyWhenUnresolved(t *testing.T) {
	app := NewApplication()
	app.SetLocale(Locale("xx"))

	const unknownKey = "app.custom.greeting"
	if got := app.T(unknownKey); got != unknownKey {
		t.Errorf("T(%q) = %q, want the key itself as a last-resort fallback", unknownKey, got)
	}
}

// A locale with no prepared Catalog falls back to English rather than
// producing blank UI text.
func TestApplication_UnknownLocaleFallsBackToEnglish(t *testing.T) {
	app := NewApplication()
	app.SetLocale(Locale("xx"))

	if got := app.T(KeyOK); got != "OK" {
		t.Errorf("T(KeyOK) for an unregistered locale = %q, want English fallback %q", got, "OK")
	}
}

func TestApplication_SetTranslationsOverridesOneKeyWithoutLosingOthers(t *testing.T) {
	app := NewApplication()
	app.SetTranslations(LocaleEnglish, Catalog{KeyOK: "Confirm"})

	if got := app.T(KeyOK); got != "Confirm" {
		t.Errorf("T(KeyOK) after override = %q, want %q", got, "Confirm")
	}
	// KeyCancel was never overridden, so it must still fall back to the
	// library's own built-in English Catalog rather than going blank.
	if got := app.T(KeyCancel); got != "Cancel" {
		t.Errorf("T(KeyCancel) after an unrelated override = %q, want the built-in %q", got, "Cancel")
	}
}

func TestApplication_SetTranslationsRegistersANewLocale(t *testing.T) {
	app := NewApplication()
	app.SetLocale(Locale("eo")) // Esperanto: no prepared Catalog
	app.SetTranslations(Locale("eo"), Catalog{
		KeyYes: "Jes",
		KeyNo:  "Ne",
	})

	if got := app.T(KeyYes); got != "Jes" {
		t.Errorf("T(KeyYes) = %q, want %q", got, "Jes")
	}
	// KeyOK isn't in the custom catalog and Esperanto has no built-in one,
	// so it must fall back all the way to English.
	if got := app.T(KeyOK); got != "OK" {
		t.Errorf("T(KeyOK) = %q, want English fallback %q", got, "OK")
	}
}

// SetTranslations must merge into a locale's dictionary across repeated
// calls, not replace it — this is what lets an application register its
// own translations incrementally (a shared base, then per-screen keys)
// without each call wiping out the last one.
func TestApplication_SetTranslationsMergesAcrossCalls(t *testing.T) {
	app := NewApplication()
	app.SetTranslations(LocaleEnglish, Catalog{"myapp.title": "My App"})
	app.SetTranslations(LocaleEnglish, Catalog{"myapp.save": "Save"})

	if got := app.T("myapp.title"); got != "My App" {
		t.Errorf(`T("myapp.title") = %q, want %q (lost after a second SetTranslations call)`, got, "My App")
	}
	if got := app.T("myapp.save"); got != "Save" {
		t.Errorf(`T("myapp.save") = %q, want %q`, got, "Save")
	}

	// A key present in both calls must resolve to the later value.
	app.SetTranslations(LocaleEnglish, Catalog{"myapp.title": "My App v2"})
	if got := app.T("myapp.title"); got != "My App v2" {
		t.Errorf(`T("myapp.title") after re-registering = %q, want the newer %q`, got, "My App v2")
	}
}

// Merge must apply later Catalog values on top of earlier ones, letting a
// key repeated across them resolve to the last one's entry, exactly the
// way SetTranslations layers a later call over an earlier one.
func TestCatalog_MergeLaterOverridesEarlier(t *testing.T) {
	common := Catalog{"myapp.ok": "OK", "myapp.cancel": "Cancel"}
	screen := Catalog{"myapp.ok": "Confirm", "myapp.title": "Screen"}

	merged := common.Merge(screen)

	if merged["myapp.ok"] != "Confirm" {
		t.Errorf(`merged["myapp.ok"] = %q, want the overriding %q`, merged["myapp.ok"], "Confirm")
	}
	if merged["myapp.cancel"] != "Cancel" {
		t.Errorf(`merged["myapp.cancel"] = %q, want %q`, merged["myapp.cancel"], "Cancel")
	}
	if merged["myapp.title"] != "Screen" {
		t.Errorf(`merged["myapp.title"] = %q, want %q`, merged["myapp.title"], "Screen")
	}
}

// Merge combines more than two Catalog values, applied left to right, and
// must not mutate any of its inputs — a caller composing several package-
// level Catalog vars shouldn't have to worry about Merge corrupting one of
// them for a later, unrelated call.
func TestCatalog_MergeIsVariadicAndNonMutating(t *testing.T) {
	base := Catalog{"a": "1"}
	mid := Catalog{"b": "2"}
	top := Catalog{"a": "override", "c": "3"}

	merged := base.Merge(mid, top)

	want := Catalog{"a": "override", "b": "2", "c": "3"}
	for k, v := range want {
		if merged[k] != v {
			t.Errorf("merged[%q] = %q, want %q", k, merged[k], v)
		}
	}

	if base["a"] != "1" || len(base) != 1 {
		t.Errorf("base was mutated by Merge: %+v", base)
	}
	if top["a"] != "override" || len(top) != 2 {
		t.Errorf("top was mutated by Merge: %+v", top)
	}
}

func TestApplication_SetTranslationsAcceptsAMergedCatalog(t *testing.T) {
	common := Catalog{"myapp.ok": "OK"}
	screen := Catalog{"myapp.title": "My App"}

	app := NewApplication()
	app.SetTranslations(LocaleEnglish, common.Merge(screen))

	if got := app.T("myapp.ok"); got != "OK" {
		t.Errorf(`T("myapp.ok") = %q, want %q`, got, "OK")
	}
	if got := app.T("myapp.title"); got != "My App" {
		t.Errorf(`T("myapp.title") = %q, want %q`, got, "My App")
	}
}

// Regression: builtinCatalog must never hand back a map a caller (or a
// future SetTranslations override) could mutate and have that mutation
// bleed into a later, unrelated call.
func TestBuiltinCatalog_CallsAreIndependent(t *testing.T) {
	first := builtinCatalog(LocaleEnglish)
	first[KeyOK] = "Mutated"

	second := builtinCatalog(LocaleEnglish)
	if second[KeyOK] != "OK" {
		t.Errorf("second builtinCatalog call saw %q, want the unmutated built-in %q", second[KeyOK], "OK")
	}
}

// Integration: ShowConfirm's button labels track the Application's Locale,
// exactly like the identical-condition tests in dialogs_test.go do for the
// default English locale.
func TestShowConfirm_UsesCurrentLocale(t *testing.T) {
	app := NewApplication()
	app.SetLocale(LocaleFrench)

	ShowConfirm(app, "Supprimer", "Supprimer ce fichier ?", BtnDanger, func() {})
	btns := buttons(app.topModal())
	if len(btns) != 2 {
		t.Fatalf("got %d buttons, want 2", len(btns))
	}
	if btns[0].Text != "Oui" || btns[1].Text != "Non" {
		t.Errorf("button labels = %q, %q, want Oui, Non", btns[0].Text, btns[1].Text)
	}
}

// Integration: ShowFilePicker's column headers and Open/Cancel buttons
// track the Application's Locale too.
func TestShowFilePicker_UsesCurrentLocale(t *testing.T) {
	app := NewApplication()
	app.SetLocale(LocaleGerman)

	ShowFilePicker(app, ".", func(string) {})
	mod := app.topModal()
	if mod == nil {
		t.Fatal("ShowFilePicker did not open a modal")
	}

	btns := buttons(mod)
	if len(btns) < 2 {
		t.Fatalf("got %d buttons, want at least 2 (Open, Cancel)", len(btns))
	}
	last, secondLast := btns[len(btns)-1], btns[len(btns)-2]
	if secondLast.Text != "Öffnen" || last.Text != "Abbrechen" {
		t.Errorf("Open/Cancel buttons = %q, %q, want Öffnen, Abbrechen", secondLast.Text, last.Text)
	}

	if mod.Title != " Öffnen " {
		t.Errorf("modal title = %q, want %q", mod.Title, " Öffnen ")
	}
}
