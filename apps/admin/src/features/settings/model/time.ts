/**
 * «5 минут назад» рядом с датой в журналах.
 *
 * Лежит в `model/`, а не рядом с `formatDateTime` в `ui/parts`: здесь
 * есть что проверить — выбор единицы и граница «меньше минуты», — а
 * тест на модуль с компонентами тянул бы за собой полреактовского
 * дерева ради чистой функции.
 *
 * Точная дата отвечает «когда именно», но не отвечает «давно ли», — а
 * в журнале спрашивают обычно второе: свежая запись это или прошлогодняя.
 * Считать разницу в голове из «14.09.2026, 11:42» человек не должен.
 *
 * Считает и склоняет `Intl.RelativeTimeFormat` — он в браузере уже есть,
 * знает все три наших языка и падежи («5 минут назад», «5 daqiqa oldin»).
 * Единица выбирается самая крупная из подошедших: «вчера» полезнее,
 * чем «26 часов назад».
 *
 * Пусто — там же, где и у `formatDateTime`: на мусоре и на пустой строке.
 * Будущее не отсекаем: часы сервера и браузера расходятся, и «через
 * минуту» у свежей записи честнее молчания.
 */
const UNITS: [Intl.RelativeTimeFormatUnit, number][] = [
  ["year", 365 * 24 * 60 * 60 * 1000],
  ["month", 30 * 24 * 60 * 60 * 1000],
  ["day", 24 * 60 * 60 * 1000],
  ["hour", 60 * 60 * 1000],
  ["minute", 60 * 1000],
];

export function relativeTime(value: string, locale: string, now = Date.now()): string {
  if (!value) return "";

  const date = new Date(value);
  if (Number.isNaN(date.getTime())) return "";

  const diff = date.getTime() - now;
  const format = new Intl.RelativeTimeFormat(locale, { numeric: "auto" });

  for (const [unit, size] of UNITS) {
    const count = Math.trunc(diff / size);
    if (count !== 0) return format.format(count, unit);
  }

  /* Меньше минуты — «сейчас». Именно в секундах: тот же ноль в минутах
     даёт «в эту минуту» вместо «сейчас» (numeric: "auto" подставляет
     готовую формулировку, и у секунд она та, которую и хотели). */
  return format.format(0, "second");
}
