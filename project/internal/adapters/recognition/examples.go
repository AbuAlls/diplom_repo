package recognition

import (
	"strings"
	"time"
)

// exampleDoc is a scripted recognition result for one known example file. The
// mock recognizer matches uploads by filename and, on a hit, returns this
// individualized output so each example document shows distinct, plausible
// analytics in the UI.
type exampleDoc struct {
	recognizedText string
	org            string
	inn            string
	extNo          string
	documentDate   time.Time
	deadlines      []time.Time
	prices         []string
	productNames   []string
	quantities     []int32
	contractNums   []string
	confidence     float64

	// The following are surfaced to the frontend via structured_json so the
	// document detail screen can render an individualized summary, audit checks
	// and risk findings.
	summary string
	checks  []checkItem
	risks   []riskItem
}

// checkItem mirrors the frontend's audit-check shape: {label, ok}.
type checkItem struct {
	Label string `json:"label"`
	OK    bool   `json:"ok"`
}

// riskItem mirrors the frontend's risk shape: {label, level, color}.
type riskItem struct {
	Label string `json:"label"`
	Level string `json:"level"`
	Color string `json:"color"`
}

// CSS custom properties already defined in the frontend theme, reused so risk
// colors match the rest of the UI.
const (
	colorGreen = "var(--green)"
	colorAmber = "var(--amber)"
	colorRed   = "var(--red)"
)

func mustDate(s string) time.Time {
	t, err := time.Parse("2006-01-02", s)
	if err != nil {
		panic("recognition: bad example date " + s)
	}
	return t
}

// normalizeFileName lowercases and trims the filename so matching is forgiving
// of case and surrounding whitespace.
func normalizeFileName(name string) string {
	return strings.ToLower(strings.TrimSpace(name))
}

// exampleCatalog maps a normalized filename to its scripted analytics. The keys
// are the example files shipped in diploma front end/uploads/examples/.
var exampleCatalog = map[string]exampleDoc{
	// 1) Supply contract.
	"contract_orbita.pdf": {
		recognizedText: "Договор поставки № ПО-2026/048 между ООО «Орбита» (поставщик) и заказчиком. " +
			"Срок действия 12 месяцев с автоматической пролонгацией. Цена фиксированная, " +
			"без оговорки об индексации. Постоплата 30 календарных дней.",
		org:          "ООО «Орбита»",
		inn:          "7704123456",
		extNo:        "ПО-2026/048",
		documentDate: mustDate("2026-05-01"),
		deadlines:    []time.Time{mustDate("2027-04-30")},
		prices:       []string{"3 200 000 ₽"},
		productNames: []string{"Поставка комплектующих", "Сервисное обслуживание"},
		quantities:   []int32{1},
		contractNums: []string{"ПО-2026/048"},
		confidence:   0.96,
		summary: "Договор поставки на 12 месяцев с автоматической пролонгацией. " +
			"Выявлен пункт о фиксированной цене без индексации — возможность экономии при пересмотре.",
		checks: []checkItem{
			{Label: "Реквизиты сторон корректны", OK: true},
			{Label: "Срок действия указан", OK: true},
			{Label: "Отсутствует пункт об индексации цены", OK: false},
			{Label: "Условия расторжения определены", OK: true},
		},
		risks: []riskItem{
			{Label: "Ценовой риск (нет индексации)", Level: "Средний", Color: colorAmber},
			{Label: "Риск автопролонгации", Level: "Низкий", Color: colorGreen},
		},
	},

	// 2) Invoice.
	"invoice_alfa.pdf": {
		recognizedText: "Счёт на оплату № 1187 от ООО «Альфа-Трейд». Сумма 458 900 ₽, в том числе НДС 20%. " +
			"Срок оплаты — 5 банковских дней. Назначение: поставка офисной техники.",
		org:          "ООО «Альфа-Трейд»",
		inn:          "7715998877",
		extNo:        "1187",
		documentDate: mustDate("2026-06-03"),
		deadlines:    []time.Time{mustDate("2026-06-10")},
		prices:       []string{"458 900 ₽", "НДС 76 483 ₽"},
		productNames: []string{"Ноутбук рабочий", "МФУ лазерное", "Монитор 27\""},
		quantities:   []int32{8, 2, 8},
		contractNums: []string{},
		confidence:   0.94,
		summary: "Счёт на поставку офисной техники на 458 900 ₽. Короткий срок оплаты (5 дней) — " +
			"проверьте бюджет до подтверждения.",
		checks: []checkItem{
			{Label: "ИНН поставщика валиден", OK: true},
			{Label: "НДС выделен корректно", OK: true},
			{Label: "Банковские реквизиты присутствуют", OK: true},
			{Label: "Срок оплаты — менее 7 дней", OK: false},
		},
		risks: []riskItem{
			{Label: "Риск кассового разрыва (срок 5 дней)", Level: "Средний", Color: colorAmber},
		},
	},

	// 3) Software license.
	"license_1c.pdf": {
		recognizedText: "Лицензионный сертификат на ПО «1С:Предприятие 8». Тип лицензии — на 50 рабочих мест. " +
			"Срок действия — бессрочный. Право на обновления — до 2027-03-15 (ИТС).",
		org:          "ООО «1С-Софт»",
		inn:          "7726654321",
		extNo:        "LIC-1C-50",
		documentDate: mustDate("2026-03-15"),
		deadlines:    []time.Time{mustDate("2027-03-15")},
		prices:       []string{"690 000 ₽"},
		productNames: []string{"1С:Предприятие 8 (50 раб. мест)", "Договор ИТС ПРОФ"},
		quantities:   []int32{1},
		contractNums: []string{"LIC-1C-50"},
		confidence:   0.97,
		summary: "Лицензия 1С на 50 мест, бессрочная. Подписка на обновления (ИТС) истекает 15.03.2027 — " +
			"запланируйте продление, чтобы не потерять поддержку.",
		checks: []checkItem{
			{Label: "Лицензионный ключ читается", OK: true},
			{Label: "Количество мест соответствует штату", OK: true},
			{Label: "Подписка ИТС истекает в течение года", OK: false},
		},
		risks: []riskItem{
			{Label: "Риск окончания поддержки (ИТС)", Level: "Высокий", Color: colorRed},
			{Label: "Риск нехватки лицензий при росте штата", Level: "Низкий", Color: colorGreen},
		},
	},

	// 4) Act of completed works.
	"act_logistic.pdf": {
		recognizedText: "Акт выполненных работ № 305 от ООО «ЛогистикПро». Транспортно-экспедиционные услуги " +
			"за май 2026. Сумма 212 400 ₽. Претензий по объёму и качеству не заявлено.",
		org:          "ООО «ЛогистикПро»",
		inn:          "5009112233",
		extNo:        "305",
		documentDate: mustDate("2026-05-31"),
		deadlines:    []time.Time{mustDate("2026-06-15")},
		prices:       []string{"212 400 ₽"},
		productNames: []string{"Транспортно-экспедиционные услуги"},
		quantities:   []int32{1},
		contractNums: []string{"ТЭ-2026/12"},
		confidence:   0.93,
		summary: "Акт оказанных транспортных услуг за май на 212 400 ₽. Документ закрывает период " +
			"по договору ТЭ-2026/12; готов к оплате до 15.06.2026.",
		checks: []checkItem{
			{Label: "Ссылка на договор присутствует", OK: true},
			{Label: "Период оказания услуг указан", OK: true},
			{Label: "Подписи обеих сторон проставлены", OK: true},
		},
		risks: []riskItem{
			{Label: "Финансовый риск", Level: "Низкий", Color: colorGreen},
		},
	},

	// 5) Financial report.
	"report_q2.pdf": {
		recognizedText: "Финансовый отчёт за II квартал 2026. Выручка 18,4 млн ₽ (+12% к I кв.), " +
			"операционные расходы 13,1 млн ₽. Чистая прибыль 3,9 млн ₽. Рост дебиторской задолженности.",
		org:          "Внутренний отчёт",
		inn:          "",
		extNo:        "FIN-Q2-2026",
		documentDate: mustDate("2026-07-05"),
		deadlines:    []time.Time{},
		prices:       []string{"Выручка 18 400 000 ₽", "Прибыль 3 900 000 ₽"},
		productNames: []string{},
		quantities:   []int32{},
		contractNums: []string{},
		confidence:   0.9,
		summary: "Квартальный отчёт: выручка выросла на 12%, чистая прибыль 3,9 млн ₽. " +
			"Тревожный сигнал — рост дебиторской задолженности опережает выручку.",
		checks: []checkItem{
			{Label: "Период отчёта определён", OK: true},
			{Label: "Баланс сходится", OK: true},
			{Label: "Дебиторская задолженность растёт быстрее выручки", OK: false},
		},
		risks: []riskItem{
			{Label: "Риск ликвидности (рост дебиторки)", Level: "Средний", Color: colorAmber},
			{Label: "Риск снижения маржинальности", Level: "Низкий", Color: colorGreen},
		},
	},
}

// lookupExample returns the scripted document for a filename, if known.
func lookupExample(fileName string) (exampleDoc, bool) {
	d, ok := exampleCatalog[normalizeFileName(fileName)]
	return d, ok
}
