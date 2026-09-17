/**
 * Разбор полей журнала изменений.
 *
 * Бэкенд кладёт в `request`, `response`, `previous` и `current` СТРОКУ
 * с JSON, и каждый раз завёрнутую: `{"data": …}` (шлюз, handler.go:192
 * — он оборачивает всё, что пишет в журнал). Внутри лежит либо объект,
 * либо строка с текстом ошибки, либо пустая карта.
 *
 * Поэтому разбор один и здесь: экран показывает содержимое, а не
 * конверт, и не должен решать, какой из четырёх видов ему достался.
 */

/** Поле журнала без конверта: то, что лежало внутри `{"data": …}`. */
export function parseEntry(raw: string): unknown {
  const value = raw?.trim();
  if (!value) return undefined;

  let parsed: unknown;
  try {
    parsed = JSON.parse(value);
  } catch {
    // Не JSON — значит текст. Отдаём как есть: чаще всего это
    // сообщение об ошибке, ради которого запись и открыли.
    return value;
  }

  return typeof parsed === "object" && parsed !== null && "data" in parsed
    ? (parsed as { data: unknown }).data
    : parsed;
}

/** Содержимое поля журнала: конверт снят, JSON разложен по строкам. */
export function unwrapEntry(raw: string): string {
  const inner = parseEntry(raw);

  if (inner === null || inner === undefined) return "";
  if (typeof inner === "string") return inner;

  // Пустая карта — это «поля не было», а не «объект без ключей»:
  // шлюз подставляет её вместо отсутствующего значения.
  if (isPlainObject(inner) && !Object.keys(inner).length) return "";

  return JSON.stringify(inner, null, 2);
}

/**
 * Одно расхождение: путь до поля и что с ним стало.
 *
 * Два вида, потому что и правок два вида. У значения есть «было»
 * и «стало» — их показывают рядом. У списка ни того, ни другого нет:
 * есть то, что пропало, и то, что появилось, а остальные два десятка
 * элементов при этом не изменились, и печатать их — то же самое
 * полотно, от которого уходим. Оба списка пустые — значит набор тот же,
 * сменился только порядок.
 */
export type EntryChange =
  | { path: string; before: string; after: string }
  | { path: string; added: string[]; removed: string[] };

/**
 * Что именно поменялось: пути до разошедшихся полей.
 *
 * Ради этого журнал и открывают. Целиком «было» и «стало» — два
 * одинаковых полотна по две сотни строк, в которых отличается номер
 * порядка да один идентификатор в списке; найти их глазами нельзя,
 * а больше в записи ничего и нет.
 *
 * Разбор идёт ВГЛУБЬ, до листа: `attributes.default_limit`, а не
 * «attributes стало другим». Иначе у view, где почти всё лежит внутри
 * `attributes` и `columns`, различие снова оказывалось бы блобом.
 *
 * Списки сравниваются как МУЛЬТИМНОЖЕСТВА, а не по индексам. Вставка
 * одного элемента сдвигает все следующие, и поэлементное сравнение
 * объявило бы изменившимся весь хвост — двадцать пять ложных строк
 * вместо одной настоящей.
 *
 * Пусто — это ответ, а не отказ: у записи о чтении «было» и «стало»
 * нет вовсе, а у правки, ничего не изменившей, они совпадают.
 */
export function diffEntry(before: string, after: string): EntryChange[] {
  const left = parseEntry(before);
  const right = parseEntry(after);

  // Не объекты — значит текст ошибки или пустая карта: разбирать нечего.
  if (!isPlainObject(left) && !isPlainObject(right)) return [];

  const changes: EntryChange[] = [];
  walk("", isPlainObject(left) ? left : {}, isPlainObject(right) ? right : {}, 0, changes);
  return changes;
}

/**
 * Предел вложенности. У view настоящая глубина — четыре уровня
 * (`attributes` → `quick_filters` → элемент → поле); глубже начинается
 * не структура, а чужие данные, и путь из восьми точек читается хуже,
 * чем «эта ветка стала другой».
 */
const MAX_DEPTH = 6;

function walk(
  path: string,
  left: unknown,
  right: unknown,
  depth: number,
  changes: EntryChange[],
): void {
  if (serialize(left) === serialize(right)) return;

  if (depth < MAX_DEPTH && isPlainObject(left) && isPlainObject(right)) {
    /* Ключи объединяются и сортируются: порядок полей в ответе Go
       случаен, а список различий должен читаться одинаково всегда. */
    for (const key of [...new Set([...Object.keys(left), ...Object.keys(right)])].sort()) {
      walk(path ? `${path}.${key}` : key, left[key], right[key], depth + 1, changes);
    }
    return;
  }

  if (depth < MAX_DEPTH && Array.isArray(left) && Array.isArray(right)) {
    walkList(path, left, right, depth, changes);
    return;
  }

  changes.push({ path, before: format(left), after: format(right) });
}

function walkList(
  path: string,
  left: unknown[],
  right: unknown[],
  depth: number,
  changes: EntryChange[],
): void {
  const before = left.map(serialize);
  const after = right.map(serialize);

  const removed = missing(before, after);
  const added = missing(after, before);

  // Набор тот же — значит элементы переставили.
  if (!removed.length && !added.length) {
    changes.push({ path, added: [], removed: [] });
    return;
  }

  /*
   * Один ушёл, один пришёл, и оба — объекты: это правка на месте
   * («поменяли значение в одном фильтре»), а не замена списка. Тогда
   * полезен не сам элемент целиком, а то, чем он стал отличаться.
   */
  const [onlyAdded] = added;
  const [onlyRemoved] = removed;

  if (
    added.length === 1 &&
    removed.length === 1 &&
    onlyAdded?.startsWith("{") &&
    onlyRemoved?.startsWith("{")
  ) {
    // Номер по «стало»: он указывает на то, что человек видит сейчас.
    const index = after.indexOf(onlyAdded);
    walk(`${path}[${index}]`, JSON.parse(onlyRemoved), JSON.parse(onlyAdded), depth + 1, changes);
    return;
  }

  changes.push({ path, added: added.map(plain), removed: removed.map(plain) });
}

/**
 * Разность мультимножеств: повторы вычёркиваются по одному, иначе
 * список с двумя одинаковыми значениями всегда выглядел бы изменившимся.
 *
 * ponytail: перебором, O(n²). В `columns` десятки элементов; появятся
 * тысячи — считать через карту счётчиков.
 */
function missing(from: string[], other: string[]): string[] {
  const rest = [...other];

  return from.filter((item) => {
    const index = rest.indexOf(item);
    if (index < 0) return true;
    rest.splice(index, 1);
    return false;
  });
}

/** Ключ сравнения. Сериализация, потому что сравнивать надо и объекты. */
function serialize(value: unknown): string {
  return JSON.stringify(value) ?? "";
}

/** Элемент списка для показа: строка без кавычек, остальное как есть. */
function plain(item: string): string {
  if (!item.startsWith('"')) return item;

  const value: unknown = JSON.parse(item);
  return typeof value === "string" ? value : item;
}

function isPlainObject(value: unknown): value is Record<string, unknown> {
  return typeof value === "object" && value !== null && !Array.isArray(value);
}

/** Значение поля строкой. Пусто значит «поля не было» — так его и видно. */
function format(value: unknown): string {
  if (value === undefined) return "";
  if (typeof value === "string") return value;
  return JSON.stringify(value, null, 2) ?? "";
}

