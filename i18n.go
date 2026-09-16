package Graphite

import "fmt"

// Locale identifies a UI language as a BCP 47 language tag (e.g. "en",
// "uk", "pt"). It is a plain string, not a closed enum: SetTranslations
// accepts any Locale value, including one the library ships no prepared
// Catalog for, so a caller is never limited to the built-in languages.
type Locale string

// Built-in locales the library ships a prepared Catalog for (see
// builtinCatalog). These constants exist for convenience and typo safety
// when selecting one of them; any other Locale value works too, given a
// Catalog registered via Application.SetTranslations.
const (
	LocaleEnglish    Locale = "en"
	LocaleUkrainian  Locale = "uk"
	LocaleRussian    Locale = "ru"
	LocaleGerman     Locale = "de"
	LocaleFrench     Locale = "fr"
	LocaleSpanish    Locale = "es"
	LocalePortuguese Locale = "pt"
	LocaleItalian    Locale = "it"
	LocalePolish     Locale = "pl"
	LocaleDutch      Locale = "nl"
	LocaleTurkish    Locale = "tr"
	LocaleCzech      Locale = "cs"
	LocaleJapanese   Locale = "ja"
	LocaleChinese    Locale = "zh"
	LocaleKorean     Locale = "ko"
)

// Catalog maps a translation key to display text in one Locale. A key
// missing from a Catalog is not an error: Application.T falls back to the
// library's own built-in Catalog for that Locale, then to English, so a
// caller only needs to supply the keys it wants to override.
//
// This is the same type — and the same key->text convention — an
// application uses to translate its own UI: define your own keys (see the
// Key* constants below for the library's own naming pattern; namespacing
// your own as "<yourapp>.<area>.<element>", e.g. "myapp.toolbar.save",
// avoids ever colliding with a "graphite.*" key), call Application.T with
// them exactly like the library's own code does, and supply a Catalog per
// Locale via SetTranslations. There is no separate mechanism for "the
// library's strings" versus "your program's strings" — one Application,
// one Locale, one T, resolving both through the same per-locale Catalog.
//
// The recommended way to define one is a plain Go source file — a package-
// level Catalog literal, the same way builtinCatalogs (in this file) does
// it for the library's own strings — rather than a runtime-loaded data
// file: `go build`/`go vet` reject a duplicate key or a malformed literal
// before the program ever runs, the translation compiles straight into
// the binary with no separate file to ship or possibly go missing at
// runtime, and it cross-compiles for free along with everything else. See
// "Translating your own application" in docs/i18n.md for the recommended
// layout (one file per language under a locales package) and Merge below
// for composing several Catalog values into one.
type Catalog map[string]string

// Merge returns a new Catalog containing c's entries with each of others
// applied on top, in the order given — a later Catalog's entry for a key
// overrides an earlier one's, including one of c's own. Neither c nor any
// of others is modified.
//
// This is how several Go-native Catalog values — e.g. one per file under
// an application's locales package — compose into one dictionary before a
// single Application.SetTranslations call, entirely through ordinary Go
// values and function calls: a typo in a variable name or an accidental
// duplicate key inside one literal is a compile error, not something
// discovered by running the program.
func (c Catalog) Merge(others ...Catalog) Catalog {
	merged := make(Catalog, len(c))
	for k, v := range c {
		merged[k] = v
	}
	for _, other := range others {
		for k, v := range other {
			merged[k] = v
		}
	}
	return merged
}

// Translation keys for every string the library's own built-in dialogs
// (dialogs.go, app.go's ShowMessage), file picker (filepicker.go), and
// value editor (fader.go's ShowValueEditor) display. A custom Catalog
// passed to Application.SetTranslations may reuse these keys to override
// a prepared translation, or define its own keys for a caller's own
// widget text — T resolves any key the same way (see Catalog).
const (
	KeyYes             = "graphite.yes"
	KeyNo              = "graphite.no"
	KeyOK              = "graphite.ok"
	KeyCancel          = "graphite.cancel"
	KeyValueRequired   = "graphite.value_required"
	KeyOpen            = "graphite.open"
	KeyDirLabel        = "graphite.filepicker.dir_label"
	KeyFileNameLabel   = "graphite.filepicker.filename_label"
	KeyColumnName      = "graphite.filepicker.column_name"
	KeyColumnDate      = "graphite.filepicker.column_date"
	KeyColumnType      = "graphite.filepicker.column_type"
	KeyColumnSize      = "graphite.filepicker.column_size"
	KeyFileFolder      = "graphite.filepicker.file_folder"
	KeyFileGeneric     = "graphite.filepicker.file_generic"
	KeyFilterAll       = "graphite.filepicker.filter_all"
	KeyFilterGph       = "graphite.filepicker.filter_gph"
	KeyFilterImage     = "graphite.filepicker.filter_image"
	KeyFilterVideo     = "graphite.filepicker.filter_video"
	KeyValuePrompt     = "graphite.valueeditor.prompt"
	KeyValueRangeError = "graphite.valueeditor.range_error"
)

// T resolves key to display text for app's current Locale (see SetLocale),
// formatting it with args via fmt.Sprintf when any are given — so a
// translated string can carry the same %-verbs as the English source
// (KeyValuePrompt and KeyValueRangeError both take two %g values).
//
// Resolution order: a Catalog registered for the current locale via
// SetTranslations, then the library's own prepared Catalog for that
// locale, then the same two steps for English, then key itself. A missing
// translation therefore degrades to readable English (or, for a caller's
// own custom key with no English entry either, to the key string) rather
// than an empty or garbled result.
func (app *Application) T(key string, args ...any) string {
	text := app.translate(app.locale, key)
	if text == "" && app.locale != LocaleEnglish {
		text = app.translate(LocaleEnglish, key)
	}
	if text == "" {
		text = key
	}
	if len(args) == 0 {
		return text
	}
	return fmt.Sprintf(text, args...)
}

// translate looks up key for l, preferring a Catalog registered via
// SetTranslations over the built-in one, and returns "" when neither has
// an entry.
func (app *Application) translate(l Locale, key string) string {
	if custom, ok := app.catalogs[l]; ok {
		if text, ok := custom[key]; ok {
			return text
		}
	}
	return builtinCatalog(l)[key]
}

// Locale returns the Locale currently used by T, and therefore by every
// built-in dialog, the file picker, and the value editor. Defaults to
// LocaleEnglish.
func (app *Application) Locale() Locale {
	return app.locale
}

// SetLocale changes the Locale T resolves against going forward. It does
// not itself redraw anything already on screen — a widget or dialog picks
// up the new locale the next time it calls T, e.g. the next time one of
// the Show* helpers opens a new modal.
func (app *Application) SetLocale(l Locale) {
	app.locale = l
}

// SetTranslations merges catalog into the Catalog T consults first for
// locale, before falling back to the library's own prepared Catalog (if
// any) and then to English. A key already registered for locale — from an
// earlier SetTranslations call, or from the library's own built-in
// Catalog — is overridden by catalog's entry; every other key registered
// earlier is kept. Calling it repeatedly is therefore how a program
// builds up one Locale's dictionary incrementally: once for a shared
// base, again per screen or module, again for a late override — nothing
// already registered is ever discarded except a key catalog itself
// redefines. To supply a full application translation, either make one
// SetTranslations call with every key, or split it across several calls
// (Catalog.Merge composes several Go-native Catalog values into one first,
// if you'd rather make a single call) however the codebase is organized.
func (app *Application) SetTranslations(locale Locale, catalog Catalog) {
	if app.catalogs == nil {
		app.catalogs = make(map[Locale]Catalog)
	}
	existing := app.catalogs[locale]
	merged := make(Catalog, len(existing)+len(catalog))
	for k, v := range existing {
		merged[k] = v
	}
	for k, v := range catalog {
		merged[k] = v
	}
	app.catalogs[locale] = merged
}

// builtinCatalog returns the library's prepared Catalog for l, or nil if l
// isn't one of the locales in builtinCatalogs. It is a plain function
// rather than a package-level table so no mutable shared state exists
// anywhere in the package: every call builds its own fresh map, and two
// concurrent callers (or a caller that mutates the map it gets back) can
// never observe or corrupt each other's data.
func builtinCatalog(l Locale) Catalog {
	return builtinCatalogs()[l]
}

// builtinCatalogs is the library's complete set of prepared translations,
// keyed by Locale. See the Key* constants above for what each key means;
// every prepared Catalog defines all of them.
func builtinCatalogs() map[Locale]Catalog {
	return map[Locale]Catalog{
		LocaleEnglish: {
			KeyYes:             "Yes",
			KeyNo:              "No",
			KeyOK:              "OK",
			KeyCancel:          "Cancel",
			KeyValueRequired:   "Value cannot be empty.",
			KeyOpen:            "Open",
			KeyDirLabel:        "Dir: ",
			KeyFileNameLabel:   "File name: ",
			KeyColumnName:      "Name",
			KeyColumnDate:      "Date Modified",
			KeyColumnType:      "Type",
			KeyColumnSize:      "Size",
			KeyFileFolder:      "File folder",
			KeyFileGeneric:     "File",
			KeyFilterAll:       "All Files (*.*)",
			KeyFilterGph:       "GPH Files (*.gph)",
			KeyFilterImage:     "Image Files (*.jpg, *.png, *.jpeg)",
			KeyFilterVideo:     "Video Files (*.mp4, *.avi, *.mkv)",
			KeyValuePrompt:     "Value (%g-%g):",
			KeyValueRangeError: "Enter a number between %g and %g.",
		},
		LocaleUkrainian: {
			KeyYes:             "Так",
			KeyNo:              "Ні",
			KeyOK:              "ОК",
			KeyCancel:          "Скасувати",
			KeyValueRequired:   "Значення не може бути порожнім.",
			KeyOpen:            "Відкрити",
			KeyDirLabel:        "Тека: ",
			KeyFileNameLabel:   "Ім'я файлу: ",
			KeyColumnName:      "Ім'я",
			KeyColumnDate:      "Дата зміни",
			KeyColumnType:      "Тип",
			KeyColumnSize:      "Розмір",
			KeyFileFolder:      "Папка з файлами",
			KeyFileGeneric:     "Файл",
			KeyFilterAll:       "Усі файли (*.*)",
			KeyFilterGph:       "Файли GPH (*.gph)",
			KeyFilterImage:     "Файли зображень (*.jpg, *.png, *.jpeg)",
			KeyFilterVideo:     "Відеофайли (*.mp4, *.avi, *.mkv)",
			KeyValuePrompt:     "Значення (%g-%g):",
			KeyValueRangeError: "Введіть число від %g до %g.",
		},
		LocaleRussian: {
			KeyYes:             "Да",
			KeyNo:              "Нет",
			KeyOK:              "ОК",
			KeyCancel:          "Отмена",
			KeyValueRequired:   "Значение не может быть пустым.",
			KeyOpen:            "Открыть",
			KeyDirLabel:        "Папка: ",
			KeyFileNameLabel:   "Имя файла: ",
			KeyColumnName:      "Имя",
			KeyColumnDate:      "Дата изменения",
			KeyColumnType:      "Тип",
			KeyColumnSize:      "Размер",
			KeyFileFolder:      "Папка с файлами",
			KeyFileGeneric:     "Файл",
			KeyFilterAll:       "Все файлы (*.*)",
			KeyFilterGph:       "Файлы GPH (*.gph)",
			KeyFilterImage:     "Файлы изображений (*.jpg, *.png, *.jpeg)",
			KeyFilterVideo:     "Видеофайлы (*.mp4, *.avi, *.mkv)",
			KeyValuePrompt:     "Значение (%g-%g):",
			KeyValueRangeError: "Введите число от %g до %g.",
		},
		LocaleGerman: {
			KeyYes:             "Ja",
			KeyNo:              "Nein",
			KeyOK:              "OK",
			KeyCancel:          "Abbrechen",
			KeyValueRequired:   "Der Wert darf nicht leer sein.",
			KeyOpen:            "Öffnen",
			KeyDirLabel:        "Verz.: ",
			KeyFileNameLabel:   "Dateiname: ",
			KeyColumnName:      "Name",
			KeyColumnDate:      "Geändert am",
			KeyColumnType:      "Typ",
			KeyColumnSize:      "Größe",
			KeyFileFolder:      "Dateiordner",
			KeyFileGeneric:     "Datei",
			KeyFilterAll:       "Alle Dateien (*.*)",
			KeyFilterGph:       "GPH-Dateien (*.gph)",
			KeyFilterImage:     "Bilddateien (*.jpg, *.png, *.jpeg)",
			KeyFilterVideo:     "Videodateien (*.mp4, *.avi, *.mkv)",
			KeyValuePrompt:     "Wert (%g-%g):",
			KeyValueRangeError: "Geben Sie eine Zahl zwischen %g und %g ein.",
		},
		LocaleFrench: {
			KeyYes:             "Oui",
			KeyNo:              "Non",
			KeyOK:              "OK",
			KeyCancel:          "Annuler",
			KeyValueRequired:   "La valeur ne peut pas être vide.",
			KeyOpen:            "Ouvrir",
			KeyDirLabel:        "Dossier : ",
			KeyFileNameLabel:   "Nom du fichier : ",
			KeyColumnName:      "Nom",
			KeyColumnDate:      "Date de modification",
			KeyColumnType:      "Type",
			KeyColumnSize:      "Taille",
			KeyFileFolder:      "Dossier de fichiers",
			KeyFileGeneric:     "Fichier",
			KeyFilterAll:       "Tous les fichiers (*.*)",
			KeyFilterGph:       "Fichiers GPH (*.gph)",
			KeyFilterImage:     "Fichiers image (*.jpg, *.png, *.jpeg)",
			KeyFilterVideo:     "Fichiers vidéo (*.mp4, *.avi, *.mkv)",
			KeyValuePrompt:     "Valeur (%g-%g) :",
			KeyValueRangeError: "Entrez un nombre entre %g et %g.",
		},
		LocaleSpanish: {
			KeyYes:             "Sí",
			KeyNo:              "No",
			KeyOK:              "Aceptar",
			KeyCancel:          "Cancelar",
			KeyValueRequired:   "El valor no puede estar vacío.",
			KeyOpen:            "Abrir",
			KeyDirLabel:        "Carpeta: ",
			KeyFileNameLabel:   "Nombre de archivo: ",
			KeyColumnName:      "Nombre",
			KeyColumnDate:      "Fecha de modificación",
			KeyColumnType:      "Tipo",
			KeyColumnSize:      "Tamaño",
			KeyFileFolder:      "Carpeta de archivos",
			KeyFileGeneric:     "Archivo",
			KeyFilterAll:       "Todos los archivos (*.*)",
			KeyFilterGph:       "Archivos GPH (*.gph)",
			KeyFilterImage:     "Archivos de imagen (*.jpg, *.png, *.jpeg)",
			KeyFilterVideo:     "Archivos de vídeo (*.mp4, *.avi, *.mkv)",
			KeyValuePrompt:     "Valor (%g-%g):",
			KeyValueRangeError: "Introduzca un número entre %g y %g.",
		},
		LocalePortuguese: {
			KeyYes:             "Sim",
			KeyNo:              "Não",
			KeyOK:              "OK",
			KeyCancel:          "Cancelar",
			KeyValueRequired:   "O valor não pode estar vazio.",
			KeyOpen:            "Abrir",
			KeyDirLabel:        "Pasta: ",
			KeyFileNameLabel:   "Nome do arquivo: ",
			KeyColumnName:      "Nome",
			KeyColumnDate:      "Data de modificação",
			KeyColumnType:      "Tipo",
			KeyColumnSize:      "Tamanho",
			KeyFileFolder:      "Pasta de arquivos",
			KeyFileGeneric:     "Arquivo",
			KeyFilterAll:       "Todos os arquivos (*.*)",
			KeyFilterGph:       "Arquivos GPH (*.gph)",
			KeyFilterImage:     "Arquivos de imagem (*.jpg, *.png, *.jpeg)",
			KeyFilterVideo:     "Arquivos de vídeo (*.mp4, *.avi, *.mkv)",
			KeyValuePrompt:     "Valor (%g-%g):",
			KeyValueRangeError: "Insira um número entre %g e %g.",
		},
		LocaleItalian: {
			KeyYes:             "Sì",
			KeyNo:              "No",
			KeyOK:              "OK",
			KeyCancel:          "Annulla",
			KeyValueRequired:   "Il valore non può essere vuoto.",
			KeyOpen:            "Apri",
			KeyDirLabel:        "Cartella: ",
			KeyFileNameLabel:   "Nome file: ",
			KeyColumnName:      "Nome",
			KeyColumnDate:      "Data modifica",
			KeyColumnType:      "Tipo",
			KeyColumnSize:      "Dimensione",
			KeyFileFolder:      "Cartella di file",
			KeyFileGeneric:     "File",
			KeyFilterAll:       "Tutti i file (*.*)",
			KeyFilterGph:       "File GPH (*.gph)",
			KeyFilterImage:     "File immagine (*.jpg, *.png, *.jpeg)",
			KeyFilterVideo:     "File video (*.mp4, *.avi, *.mkv)",
			KeyValuePrompt:     "Valore (%g-%g):",
			KeyValueRangeError: "Inserisci un numero compreso tra %g e %g.",
		},
		LocalePolish: {
			KeyYes:             "Tak",
			KeyNo:              "Nie",
			KeyOK:              "OK",
			KeyCancel:          "Anuluj",
			KeyValueRequired:   "Wartość nie może być pusta.",
			KeyOpen:            "Otwórz",
			KeyDirLabel:        "Katalog: ",
			KeyFileNameLabel:   "Nazwa pliku: ",
			KeyColumnName:      "Nazwa",
			KeyColumnDate:      "Data modyfikacji",
			KeyColumnType:      "Typ",
			KeyColumnSize:      "Rozmiar",
			KeyFileFolder:      "Folder plików",
			KeyFileGeneric:     "Plik",
			KeyFilterAll:       "Wszystkie pliki (*.*)",
			KeyFilterGph:       "Pliki GPH (*.gph)",
			KeyFilterImage:     "Pliki obrazów (*.jpg, *.png, *.jpeg)",
			KeyFilterVideo:     "Pliki wideo (*.mp4, *.avi, *.mkv)",
			KeyValuePrompt:     "Wartość (%g-%g):",
			KeyValueRangeError: "Wprowadź liczbę z zakresu od %g do %g.",
		},
		LocaleDutch: {
			KeyYes:             "Ja",
			KeyNo:              "Nee",
			KeyOK:              "OK",
			KeyCancel:          "Annuleren",
			KeyValueRequired:   "De waarde mag niet leeg zijn.",
			KeyOpen:            "Openen",
			KeyDirLabel:        "Map: ",
			KeyFileNameLabel:   "Bestandsnaam: ",
			KeyColumnName:      "Naam",
			KeyColumnDate:      "Datum gewijzigd",
			KeyColumnType:      "Type",
			KeyColumnSize:      "Grootte",
			KeyFileFolder:      "Bestandsmap",
			KeyFileGeneric:     "Bestand",
			KeyFilterAll:       "Alle bestanden (*.*)",
			KeyFilterGph:       "GPH-bestanden (*.gph)",
			KeyFilterImage:     "Afbeeldingsbestanden (*.jpg, *.png, *.jpeg)",
			KeyFilterVideo:     "Videobestanden (*.mp4, *.avi, *.mkv)",
			KeyValuePrompt:     "Waarde (%g-%g):",
			KeyValueRangeError: "Voer een getal in tussen %g en %g.",
		},
		LocaleTurkish: {
			KeyYes:             "Evet",
			KeyNo:              "Hayır",
			KeyOK:              "Tamam",
			KeyCancel:          "İptal",
			KeyValueRequired:   "Değer boş olamaz.",
			KeyOpen:            "Aç",
			KeyDirLabel:        "Klasör: ",
			KeyFileNameLabel:   "Dosya adı: ",
			KeyColumnName:      "Ad",
			KeyColumnDate:      "Değiştirilme Tarihi",
			KeyColumnType:      "Tür",
			KeyColumnSize:      "Boyut",
			KeyFileFolder:      "Dosya klasörü",
			KeyFileGeneric:     "Dosya",
			KeyFilterAll:       "Tüm Dosyalar (*.*)",
			KeyFilterGph:       "GPH Dosyaları (*.gph)",
			KeyFilterImage:     "Görüntü Dosyaları (*.jpg, *.png, *.jpeg)",
			KeyFilterVideo:     "Video Dosyaları (*.mp4, *.avi, *.mkv)",
			KeyValuePrompt:     "Değer (%g-%g):",
			KeyValueRangeError: "%g ile %g arasında bir sayı girin.",
		},
		LocaleCzech: {
			KeyYes:             "Ano",
			KeyNo:              "Ne",
			KeyOK:              "OK",
			KeyCancel:          "Zrušit",
			KeyValueRequired:   "Hodnota nesmí být prázdná.",
			KeyOpen:            "Otevřít",
			KeyDirLabel:        "Složka: ",
			KeyFileNameLabel:   "Název souboru: ",
			KeyColumnName:      "Název",
			KeyColumnDate:      "Datum změny",
			KeyColumnType:      "Typ",
			KeyColumnSize:      "Velikost",
			KeyFileFolder:      "Složka souborů",
			KeyFileGeneric:     "Soubor",
			KeyFilterAll:       "Všechny soubory (*.*)",
			KeyFilterGph:       "Soubory GPH (*.gph)",
			KeyFilterImage:     "Obrázkové soubory (*.jpg, *.png, *.jpeg)",
			KeyFilterVideo:     "Video soubory (*.mp4, *.avi, *.mkv)",
			KeyValuePrompt:     "Hodnota (%g-%g):",
			KeyValueRangeError: "Zadejte číslo mezi %g a %g.",
		},
		LocaleJapanese: {
			KeyYes:             "はい",
			KeyNo:              "いいえ",
			KeyOK:              "OK",
			KeyCancel:          "キャンセル",
			KeyValueRequired:   "値を空にすることはできません。",
			KeyOpen:            "開く",
			KeyDirLabel:        "フォルダー: ",
			KeyFileNameLabel:   "ファイル名: ",
			KeyColumnName:      "名前",
			KeyColumnDate:      "更新日時",
			KeyColumnType:      "種類",
			KeyColumnSize:      "サイズ",
			KeyFileFolder:      "ファイル フォルダー",
			KeyFileGeneric:     "ファイル",
			KeyFilterAll:       "すべてのファイル (*.*)",
			KeyFilterGph:       "GPH ファイル (*.gph)",
			KeyFilterImage:     "画像ファイル (*.jpg, *.png, *.jpeg)",
			KeyFilterVideo:     "動画ファイル (*.mp4, *.avi, *.mkv)",
			KeyValuePrompt:     "値 (%g～%g):",
			KeyValueRangeError: "%g から %g の間の数値を入力してください。",
		},
		LocaleChinese: {
			KeyYes:             "是",
			KeyNo:              "否",
			KeyOK:              "确定",
			KeyCancel:          "取消",
			KeyValueRequired:   "值不能为空。",
			KeyOpen:            "打开",
			KeyDirLabel:        "文件夹: ",
			KeyFileNameLabel:   "文件名: ",
			KeyColumnName:      "名称",
			KeyColumnDate:      "修改日期",
			KeyColumnType:      "类型",
			KeyColumnSize:      "大小",
			KeyFileFolder:      "文件夹",
			KeyFileGeneric:     "文件",
			KeyFilterAll:       "所有文件 (*.*)",
			KeyFilterGph:       "GPH 文件 (*.gph)",
			KeyFilterImage:     "图像文件 (*.jpg, *.png, *.jpeg)",
			KeyFilterVideo:     "视频文件 (*.mp4, *.avi, *.mkv)",
			KeyValuePrompt:     "值 (%g-%g)：",
			KeyValueRangeError: "请输入 %g 到 %g 之间的数字。",
		},
		LocaleKorean: {
			KeyYes:             "예",
			KeyNo:              "아니요",
			KeyOK:              "확인",
			KeyCancel:          "취소",
			KeyValueRequired:   "값을 비워 둘 수 없습니다.",
			KeyOpen:            "열기",
			KeyDirLabel:        "폴더: ",
			KeyFileNameLabel:   "파일 이름: ",
			KeyColumnName:      "이름",
			KeyColumnDate:      "수정한 날짜",
			KeyColumnType:      "형식",
			KeyColumnSize:      "크기",
			KeyFileFolder:      "파일 폴더",
			KeyFileGeneric:     "파일",
			KeyFilterAll:       "모든 파일 (*.*)",
			KeyFilterGph:       "GPH 파일 (*.gph)",
			KeyFilterImage:     "이미지 파일 (*.jpg, *.png, *.jpeg)",
			KeyFilterVideo:     "비디오 파일 (*.mp4, *.avi, *.mkv)",
			KeyValuePrompt:     "값 (%g~%g):",
			KeyValueRangeError: "%g에서 %g 사이의 숫자를 입력하세요.",
		},
	}
}
