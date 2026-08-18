import type { Field } from "@/features/table";
import { cellKind, type CellKind } from "./cell-kind";
import type { Item } from "./types";

/**
 * Значение ячейки: чтение из строки и запись обратно.
 *
 * Здесь живут все преобразования дат, и это не мелочь. В ucode три
 * временных типа с разным смыслом:
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

export type DateKind = Extract<CellKind, "date" | "datetime" | "datetime_naive">;

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

/** Значение для <input type="date"> и <input type="datetime-local">. */
export function toDateInput(value: unknown, kind: DateKind): string {
  const parsed = toDateValue(value, kind);
  if (!parsed) return "";

  const { date, naive } = parsed;
  const year = naive ? date.getUTCFullYear() : date.getFullYear();
  const month = pad((naive ? date.getUTCMonth() : date.getMonth()) + 1);
  const day = pad(naive ? date.getUTCDate() : date.getDate());
  const hour = pad(naive ? date.getUTCHours() : date.getHours());
  const minute = pad(naive ? date.getUTCMinutes() : date.getMinutes());

  return kind === "date" ? `${year}-${month}-${day}` : `${year}-${month}-${day}T${hour}:${minute}`;
}

/**
 * Обратно в то, что примет бэкенд.
 *
 * DATE_TIME_WITHOUT_TIME_ZONE разбирается им ровно двумя шаблонами
 * (helper.ConvertTimestamp2DB), и «2006-01-02T15:04:05Z» — один из них.
 * Ни миллисекунд, ни смещения там быть не должно: непонятную строку
 * он молча превращает в пустую, то есть затирает значение. Поэтому
 * toISOString() сюда не годится — он всегда добавляет миллисекунды.
 */
export function fromDateInput(text: string, kind: DateKind): string | null {
  if (!text) return null;

  if (kind === "date") return text;
  if (kind === "datetime_naive") return `${text}:00Z`;

  const date = new Date(text);
  return Number.isNaN(date.getTime()) ? null : date.toISOString();
}

/**
 * Дата из календаря → «ГГГГ-ММ-ДД».
 *
 * Части берутся местные, а не UTC: календарь отдаёт местную полночь,
 * и toISOString() у пользователя восточнее Гринвича вернул бы вчера.
 */
export function toDayInput(date: Date): string {
  return `${date.getFullYear()}-${pad(date.getMonth() + 1)}-${pad(date.getDate())}`;
}

/** «10:13:00» → «10:13» для списка часов и минут и обратно. */
export function toTimeInput(value: unknown): string {
  return typeof value === "string" ? value.slice(0, 5) : "";
}

export function fromTimeInput(text: string): string | null {
  return text ? `${text}:00` : null;
}

/** Пустой ввод — это null, а не ноль: «не заполнено» и «0» разные вещи. */
export function toNumber(text: string): number | null {
  const trimmed = text.trim().replace(",", ".");
  if (!trimmed) return null;

  const value = Number(trimmed);
  return Number.isFinite(value) ? value : null;
}

/** Значение, которого нет. Пустая строка и пустой список — тоже. */
export function isBlank(value: unknown): boolean {
  return (
    value === null ||
    value === undefined ||
    value === "" ||
    (Array.isArray(value) && value.length === 0)
  );
}

/**
 * Стоит ли отправлять правку. Все виды пустоты равны между собой: иначе
 * открытая и закрытая без единого нажатия пустая ячейка уезжала бы
 * на сервер запросом «поменять undefined на пустую строку».
 */
export function sameValue(a: unknown, b: unknown): boolean {
  if (isBlank(a) && isBlank(b)) return true;
  return JSON.stringify(a) === JSON.stringify(b);
}

/**
 * Новая запись: guid и значения по умолчанию из настроек полей.
 *
 * Подставляет их фронт, а не база. Колонки `default` у поля нет — значение
 * лежит в свободном мешке attributes (`defaultValue`, прежнее имя того же
 * ключа — `default_values`), и вставка о нём не знает ничего. Не подставим
 * мы — не подставит никто.
 *
 * Собирается в одном месте, потому что запись заводится в двух: строкой
 * в подвале таблицы и карточкой сбоку. Настройка, которая работает
 * в одном из них, хуже отсутствующей.
 *
 * Значение хранится строкой, а колонка — нет: в числовую поедет число,
 * в булеву — булево. Строка в колонке NUMERIC — это 500 в ответ
 * на вставку, а не «поле осталось пустым».
 */
export function blankItem(columns: Field[]): Item {
  const row: Item = { guid: crypto.randomUUID() };

  for (const field of columns) {
    const raw = field.attributes["defaultValue"] ?? field.attributes["default_values"];
    // Список — это значение по умолчанию у MULTISELECT и связей, которые
    // мы не настраиваем; чужую настройку читаем не глядя только в мусор.
    if (typeof raw !== "string" && typeof raw !== "number" && typeof raw !== "boolean") continue;

    const text = String(raw).trim();
    if (!text) continue;

    switch (cellKind(field.type)) {
      case "number": {
        const value = toNumber(text);
        if (value !== null) row[field.slug] = value;
        break;
      }
      case "boolean": {
        if (text === "true" || text === "false") row[field.slug] = text === "true";
        break;
      }
      default:
        row[field.slug] = text;
    }
  }

  return row;
}

/** Значение MULTISELECT: в строке список, но одиночная строка тоже бывает. */
export function toList(value: unknown): string[] {
  if (Array.isArray(value)) return value.filter((item) => !isBlank(item)).map(String);
  return isBlank(value) ? [] : [String(value)];
}

function pad(value: number): string {
  return String(value).padStart(2, "0");
}
