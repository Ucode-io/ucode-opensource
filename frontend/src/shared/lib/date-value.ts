/**
 * Разбор и показ дат — одно место на всё приложение.
 *
 * Лежит в shared, потому что читателей два и они по разные стороны
 * границы фич: ячейка (features/item) и подпись связанной строки
 * (shared/lib/relation-label, её зовут и item, и table). Две копии
 * этого разбора однажды показали бы одну и ту же дату по-разному.
 *
 * В ucode три временных типа с разным смыслом:
 *
 *   DATE                          календарная дата, колонка DATE
 *   DATE_TIME                     момент времени, колонка TIMESTAMPTZ
 *   DATE_TIME_WITHOUT_TIME_ZONE   настенные часы, колонка TIMESTAMP
 *
 * Только средний из них — момент. Два других часового пояса не имеют,
 * и прогонять их через `new Date(value)` нельзя: строка «2026-01-06»
 * разбирается как полночь UTC, а в Нью-Йорке это ещё 5 января. День
 * съезжает у половины планеты и только на части значений — такую ошибку
 * ищут неделями.
 */

export type DateKind = "date" | "datetime" | "datetime_naive";

/** Дата без пояса: разбираем как текст, а не как момент времени. */
const NAIVE = /^(\d{4})-(\d{2})-(\d{2})(?:[T ](\d{2}):(\d{2}))?/;

/**
 * Тот же смысл, но задом наперёд: «24.12.2025 08:49».
 *
 * Это не причуда данных, а формат ответа. DATE_TIME_WITHOUT_TIME_ZONE
 * бэкенд отдаёт именно так (items.go: `Format(config.TimeLayoutItems)`),
 * хотя в колонке лежит обычный timestamp. Разобрать его как момент
 * времени невозможно: `new Date("24.12.2025 08:49")` — Invalid Date,
 * и без этой ветки поле показывалось бы сырой строкой.
 */
const DOTTED = /^(\d{2})\.(\d{2})\.(\d{4})(?:[T ](\d{2}):(\d{2}))?/;

export type DateValue = {
  date: Date;
  /**
   * Печатать в UTC. У значения без пояса части даты положены в Date
   * как UTC — так же их и надо читать обратно, иначе сдвиг вернётся.
   */
  naive: boolean;
};

export function toDateValue(value: unknown, kind: DateKind): DateValue | null {
  if (typeof value !== "string" || !value.trim()) return null;

  if (kind === "datetime") {
    const date = new Date(value);
    return Number.isNaN(date.getTime()) ? null : { date, naive: false };
  }

  const iso = NAIVE.exec(value);
  const dotted = iso ? null : DOTTED.exec(value);

  // Не наш вид строки — пробуем разобрать как момент. Показать значение
  // приблизительно лучше, чем не показать вовсе.
  if (!iso && !dotted) {
    const date = new Date(value);
    return Number.isNaN(date.getTime()) ? null : { date, naive: false };
  }

  const [year, month, day, hour = "00", minute = "00"] = iso
    ? iso.slice(1)
    : [dotted![3], dotted![2], dotted![1], dotted![4], dotted![5]];

  return {
    date: new Date(Date.UTC(Number(year), Number(month) - 1, Number(day), Number(hour), Number(minute))),
    naive: true,
  };
}

/**
 * Даты форматируются по языку интерфейса. Форматтер кэшируется: Intl
 * стоит дорого, а в таблице ячейки считаются сотнями.
 *
 * Пояс — часть ключа: значение без пояса печатается в UTC, иначе браузер
 * пересчитает его в местное время и сдвинет то, что сдвигать нельзя.
 */
const formatters = new Map<string, Intl.DateTimeFormat>();

export function dateFormatter(locale: string, withTime: boolean, utc: boolean): Intl.DateTimeFormat {
  const key = `${locale}:${withTime}:${utc}`;
  let cached = formatters.get(key);

  if (!cached) {
    cached = new Intl.DateTimeFormat(locale, {
      dateStyle: "medium",
      ...(withTime ? { timeStyle: "short" as const } : {}),
      ...(utc ? { timeZone: "UTC" } : {}),
    });
    formatters.set(key, cached);
  }

  return cached;
}

/** Готовая подпись даты. Не разобралось — null, показывать нечего. */
export function formatDate(value: unknown, kind: DateKind, locale: string): string | null {
  const parsed = toDateValue(value, kind);
  if (!parsed) return null;

  return dateFormatter(locale, kind !== "date", parsed.naive).format(parsed.date);
}
