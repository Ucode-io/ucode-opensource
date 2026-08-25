import type { Field } from "@/features/table";
import { toDateValue, type DateKind } from "@/shared/lib/date-value";
import { cellKind } from "./cell-kind";
import type { Item } from "./types";

/**
 * Значение ячейки: чтение из строки и запись обратно.
 *
 * Разбор дат живёт этажом ниже — `shared/lib/date-value`: ту же дату
 * показывает подпись связанной строки, а она общая для двух фич.
 * Здесь остаётся то, что относится к ПРАВКЕ: значение для поля ввода
 * и обратно в то, что примет бэкенд.
 */

export { toDateValue, type DateKind, type DateValue } from "@/shared/lib/date-value";

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

/*
 * Разметка → текст, для MULTI_LINE.
 *
 * В ucode это поле заполняется редактором ReactQuill, и в колонку
 * уезжает HTML («<p>Текст</p>»). Показать его тегами — значит показать
 * не то, что человек написал; старая админка поэтому и печатала
 * `stripHtmlTags(value)`.
 *
 * Условие входа — ЗАКРЫВАЮЩИЙ или самозакрытый тег. Старая снимает
 * `<…>` безусловно и на «2 < 3 > 1» съедает середину строки; такое
 * значение здесь остаётся как есть.
 *
 * Разбор строкой, а не DOMParser: то же самое в пять строк, и его
 * можно проверить тестом, не поднимая DOM ради одной функции.
 * Сущностей ровно те шесть, что пишет редактор.
 */
const MARKUP = /<\/[a-z][^>]*>|<[a-z][^>]*\/>/i;

const ENTITIES: Record<string, string> = {
  amp: "&",
  lt: "<",
  gt: ">",
  quot: '"',
  apos: "'",
  nbsp: " ",
};

export function plainText(value: string): string {
  if (!MARKUP.test(value)) return value;

  return value
    // Абзац и перенос — это перенос строки, а не склейка слов.
    .replace(/<\/(p|div|li|h[1-6]|tr)\s*>|<br\s*\/?>/gi, "\n")
    .replace(/<[^>]*>/g, "")
    .replace(/&([a-z]+);/gi, (whole, name: string) => ENTITIES[name.toLowerCase()] ?? whole)
    .trim();
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
