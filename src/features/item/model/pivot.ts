import type { Item } from "./types";

/**
 * Сводная таблица: строки собираются в клетки «значение поля × значение
 * поля», а в клетке — число.
 *
 * Считает КЛИЕНТ, по загруженным строкам. Серверной сводки нет: ветка
 * группировки собирает таблицу без `WHERE` и `LIMIT` (см. backend-notes,
 * «Группировка»), а `view.summaries` не считает ни одна ручка. Поэтому
 * сводная у нас — инструмент разбора загруженной выборки, и экран обязан
 * говорить об этом прямо: «посчитано по N загруженным строкам».
 *
 * Из этого же следует, что настройки сводной живут в адресе, а не в view:
 * это не то, что админ настроил всем, а то, как человек сейчас смотрит.
 *
 * Полей строк НЕСКОЛЬКО — как в старой админке, где `rowGroup` ставился
 * по всему списку `group_by_columns` (`usePivotProps.jsx:183`), а ag-grid
 * складывал их во вложенные группы. У нас то же самое своими руками:
 * дерево уровней, у каждого узла — подытог по своему поддереву.
 */

/** Чем сводят значения в клетке. Порядок — как в переключателе. */
export const AGGREGATIONS = ["count", "sum", "avg", "min", "max"] as const;

export type Aggregation = (typeof AGGREGATIONS)[number];

export function toAggregation(value: string | undefined): Aggregation {
  return AGGREGATIONS.find((item) => item === value) ?? "count";
}

/**
 * По чему отсортированы строки.
 *
 * `label` — по подписи, `total` — по итогу строки, `column` — по клетке
 * в конкретной колонке. Разными вариантами, а не строкой с условным
 * значением: ключ колонки — это ЗНАЧЕНИЕ ПОЛЯ, и «total» в нём вполне
 * может оказаться настоящим статусом.
 */
export type PivotSort = {
  by: { kind: "label" } | { kind: "total" } | { kind: "column"; key: string };
  desc: boolean;
};

export const DEFAULT_SORT: PivotSort = { by: { kind: "label" }, desc: false };

export type PivotSetup = {
  /**
   * Поля строк, сверху вниз: каждое следующее — вложенный уровень.
   * Пусто — сводить нечего.
   */
  rowSlugs: string[];
  /** Поле колонок. Пусто — одна колонка «Всего». */
  colSlug: string;
  /** Поле, по которому считают. Для `count` не нужно. */
  valueSlug: string;
  aggregation: Aggregation;
  sort: PivotSort;
  /**
   * Не считать строки, у которых поле строк или колонок пустое.
   *
   * Именно НЕ СЧИТАТЬ, а не спрятать посчитанное: спрятанная колонка,
   * которая всё ещё сидит в «Итого», — это числа, которые не сходятся
   * с тем, что видно на экране. Сколько строк выпало, видно по счётчику
   * в шапке.
   */
  skipEmpty: boolean;
};

/** Строка сводной. Уровень задаёт отступ, путь — сворачивание. */
export type PivotRow = {
  /** Ключи уровней сверху вниз: `["Москва", "Иванов"]`. */
  path: string[];
  /** Подпись — ключ своего уровня. */
  key: string;
  level: number;
  /** Есть ли вложенные строки: у листа сворачивать нечего. */
  hasChildren: boolean;
  /** Клетки по колонкам, сведённые по ВСЕМУ поддереву строки. */
  cells: Map<string, number>;
  total: number;
};

export type PivotTable = {
  /** Строки всех уровней, уже отсортированные, обходом сверху вниз. */
  rows: PivotRow[];
  /** Значения поля колонок. Пусто — сводка в один столбец. */
  columns: string[];
  columnTotals: Map<string, number>;
  total: number;
  /** Сколько строк вошло в расчёт: им и подписан экран. */
  count: number;
};

/** Пустое значение — своя клетка, а не пропуск: «без статуса» тоже группа. */
export const EMPTY_KEY = "";

/**
 * Значение поля → ключ клетки.
 *
 * Списки (MULTISELECT) склеиваются в одну строку целиком, а не
 * раскладываются по клеткам: строка с двумя тегами попала бы в две
 * клетки сразу, и сумма по колонке перестала бы сходиться с итогом.
 */
export function pivotKey(value: unknown): string {
  if (value === null || value === undefined) return EMPTY_KEY;
  if (Array.isArray(value)) return value.map((item) => String(item ?? "")).join(", ");

  return String(value);
}

/** Узел дерева строк. Значения копятся у КАЖДОГО предка — отсюда подытоги. */
type Node = {
  key: string;
  path: string[];
  children: Map<string, Node>;
  byColumn: Map<string, number[]>;
  all: number[];
};

function node(key: string, path: string[]): Node {
  return { key, path, children: new Map(), byColumn: new Map(), all: [] };
}

export function pivotTable(rows: Item[], setup: PivotSetup): PivotTable {
  const slugs = setup.rowSlugs.filter(Boolean);
  const table: PivotTable = {
    rows: [],
    columns: [],
    columnTotals: new Map(),
    total: 0,
    count: 0,
  };

  if (!slugs.length) return table;

  const root = node("", []);
  const columns = new Set<string>();

  for (const row of rows) {
    const columnKey = setup.colSlug ? pivotKey(row[setup.colSlug]) : EMPTY_KEY;

    /*
     * Для count берётся сама строка (единица), для остального — число
     * из поля. Не число — строка в расчёт не идёт: сумма по текстовому
     * полю это не ноль, а «здесь нечего складывать».
     */
    const value = setup.aggregation === "count" ? 1 : toNumber(row[setup.valueSlug]);
    if (value === null) continue;

    const keys = slugs.map((slug) => pivotKey(row[slug]));

    /*
     * Пустая колонка проверяется только когда поле колонок ЗАДАНО: без
     * него ключ пуст у всех строк, и «без пустых» опустошило бы сводную.
     */
    if (setup.skipEmpty) {
      const blank = keys.some((key) => key === EMPTY_KEY);
      if (blank || (setup.colSlug && columnKey === EMPTY_KEY)) continue;
    }

    if (setup.colSlug) columns.add(columnKey);

    // Значение кладётся В КАЖДЫЙ узел пути, включая корень: подытог
    // группы — это свод по её поддереву, а не сумма подытогов детей
    // (для min, max и среднего это разные числа).
    let current = root;
    push(current, columnKey, value);

    for (const key of keys) {
      const path = [...current.path, key];

      let child = current.children.get(key);
      if (!child) {
        child = node(key, path);
        current.children.set(key, child);
      }

      push(child, columnKey, value);
      current = child;
    }

    table.count += 1;
  }

  // Колонки отсортированы, а не в порядке появления: порядок клеток
  // не должен зависеть от того, в каком порядке приехали строки.
  table.columns = [...columns].sort(compareKeys);

  for (const [columnKey, values] of root.byColumn) {
    table.columnTotals.set(columnKey, reduce(values, setup.aggregation));
  }
  table.total = reduce(root.all, setup.aggregation);

  collect(root, setup, table.rows);
  return table;
}

function push(target: Node, columnKey: string, value: number) {
  target.byColumn.set(columnKey, [...(target.byColumn.get(columnKey) ?? []), value]);
  target.all.push(value);
}

/** Дети узла — сведённые, отсортированные и разложенные в плоский список. */
function collect(parent: Node, setup: PivotSetup, out: PivotRow[]) {
  const children = [...parent.children.values()].map((child) => ({
    node: child,
    row: rowOf(child, setup.aggregation),
  }));

  children.sort((a, b) => compareRows(a.row, b.row, setup.sort));

  for (const child of children) {
    out.push(child.row);
    collect(child.node, setup, out);
  }
}

function rowOf(source: Node, aggregation: Aggregation): PivotRow {
  const cells = new Map<string, number>();
  for (const [columnKey, values] of source.byColumn) {
    cells.set(columnKey, reduce(values, aggregation));
  }

  return {
    path: source.path,
    key: source.key,
    level: source.path.length - 1,
    hasChildren: source.children.size > 0,
    cells,
    total: reduce(source.all, aggregation),
  };
}

function compareRows(a: PivotRow, b: PivotRow, sort: PivotSort): number {
  const direction = sort.desc ? -1 : 1;

  if (sort.by.kind === "label") {
    // Разворот применяется ПОСЛЕ проверки на «пусто»: иначе при убывании
    // отсутствие значения оказывалось бы первой строкой сводной.
    return emptyLast(a.key, b.key) ?? compareKeys(a.key, b.key) * direction;
  }

  const key = sort.by.kind === "column" ? sort.by.key : null;
  /*
   * Клетки без значения меньше любого числа: при возрастании они
   * оказываются вверху, при убывании — внизу. Это честнее, чем считать
   * их нулём: ноль означал бы «посчитали и вышло ноль».
   */
  const left = (key === null ? a.total : a.cells.get(key)) ?? -Infinity;
  const right = (key === null ? b.total : b.cells.get(key)) ?? -Infinity;

  // Равные числа разводим подписью, иначе порядок зависел бы от того,
  // в каком порядке приехали строки.
  return left === right ? compareKeys(a.key, b.key) : (left - right) * direction;
}

/**
 * Порядок ключей: числа как числа, остальное — по алфавиту с учётом языка.
 *
 * «Пусто» всегда последнее, в любую сторону: это не значение, а его
 * отсутствие, и место ему в хвосте, а не между «Астаной» и «Бишкеком».
 */
export function compareKeys(a: string, b: string): number {
  const empty = emptyLast(a, b);
  if (empty !== null) return empty;

  const left = Number(a);
  const right = Number(b);
  if (a.trim() && b.trim() && Number.isFinite(left) && Number.isFinite(right)) {
    return left - right;
  }

  return a.localeCompare(b);
}

/**
 * «Пусто» против всего остального: null — оба непустые, сравнивать
 * их надо обычным способом.
 */
function emptyLast(a: string, b: string): number | null {
  if (a !== EMPTY_KEY && b !== EMPTY_KEY) return null;
  return a === b ? 0 : a === EMPTY_KEY ? 1 : -1;
}

/**
 * Свести набор значений одним правилом.
 *
 * Среднее — по числу значений в наборе, а не по числу строк таблицы:
 * строки без числа в расчёт не попали вовсе.
 */
function reduce(values: number[], aggregation: Aggregation): number {
  if (!values.length) return 0;

  switch (aggregation) {
    case "count":
    case "sum":
      return values.reduce((total, value) => total + value, 0);
    case "avg":
      return values.reduce((total, value) => total + value, 0) / values.length;
    case "min":
      return Math.min(...values);
    case "max":
      return Math.max(...values);
  }
}

/** Число из значения поля. Не число — null: такую строку пропускаем. */
function toNumber(value: unknown): number | null {
  if (typeof value === "number") return Number.isFinite(value) ? value : null;
  if (typeof value === "string" && value.trim()) {
    const parsed = Number(value);
    return Number.isFinite(parsed) ? parsed : null;
  }
  return null;
}

/**
 * Сортировка в адресе одной строкой: `label`, `total` или `c:<ключ>`,
 * с минусом впереди для убывания.
 *
 * Приставка `c:` обязательна: без неё колонка со значением «total»
 * читалась бы как сортировка по итогу.
 */
export function formatPivotSort(sort: PivotSort): string {
  const body =
    sort.by.kind === "column" ? `c:${sort.by.key}` : sort.by.kind === "total" ? "total" : "label";

  return sort.desc ? `-${body}` : body;
}

export function parsePivotSort(value: string | undefined): PivotSort {
  if (!value) return DEFAULT_SORT;

  const desc = value.startsWith("-");
  const body = desc ? value.slice(1) : value;

  if (body === "total") return { by: { kind: "total" }, desc };
  if (body.startsWith("c:")) return { by: { kind: "column", key: body.slice(2) }, desc };

  return { by: { kind: "label" }, desc };
}

/** Одинаковая ли цель сортировки — по ней заголовок рисует стрелку. */
export function sameSortTarget(a: PivotSort["by"], b: PivotSort["by"]): boolean {
  if (a.kind !== b.kind) return false;
  return a.kind === "column" && b.kind === "column" ? a.key === b.key : true;
}

/**
 * Свёрнутые группы прячут потомков. Путь к строке начинается с пути
 * свёрнутого предка — значит, строка не видна.
 */
export function isHidden(path: string[], collapsed: ReadonlySet<string>): boolean {
  for (let depth = 1; depth < path.length; depth += 1) {
    if (collapsed.has(pathKey(path.slice(0, depth)))) return true;
  }
  return false;
}

/**
 * Путь строкой — им помечают свёрнутые группы.
 *
 * Разделитель — символ, которого нет в значениях полей: ключ клетки
 * это произвольный текст, и «Москва / Иванов» с обычным разделителем
 * столкнулся бы с настоящим значением «Москва / Иванов».
 */
export function pathKey(path: string[]): string {
  return path.join("\u0000");
}
