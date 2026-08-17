import { z } from "zod";

/**
 * Что показываем: страница, сортировка, фильтры, поиск. Всё это живёт
 * в адресе, поэтому сюда приходит уже разобранным, а здесь превращается
 * в тело запроса.
 *
 * Чистая функция и отдельный файл: форма тела нетривиальная (сортировка
 * числом, фильтры плоскими ключами вперемешку со служебными полями),
 * и ошибка в ней тихая — таблица просто показывает не те строки.
 */

export type SortDirection = "asc" | "desc";

export type Sort = {
  /** Слаг поля. Именно слаг: сортировка идёт по колонке в базе. */
  field: string;
  direction: SortDirection;
};

/**
 * Условия, которые бэкенд действительно умеет. Проверено запросами
 * к /v2/object/get-list — то, чего здесь нет, отвечает 500:
 *
 *   any        ["todo"]            любое из списка          10 → 2
 *   contains   "asd"               вхождение подстроки      10 → 6
 *   is         {$in: ["asdf"]}     точное совпадение        10 → 5, "asd" → 0
 *   equals     true                равенство
 *   between    {$gte, $lte}        диапазон                 10 → 3
 *   after      {$gt}                                        10 → 4
 *   before     {$lt}                                        10 → 3
 *
 * `$ne`, `$nin` и `$exists` не поддержаны (500), поэтому условий
 * «не равно» и «пусто» в интерфейсе нет — обещать их нельзя.
 */
export const FILTER_OPERATORS = [
  "any",
  "contains",
  "is",
  "equals",
  "between",
  "after",
  "before",
] as const;

export type FilterOperator = (typeof FILTER_OPERATORS)[number];

/**
 * Один фильтр: условие и его аргументы.
 *
 * Аргументы всегда списком строк, каким бы ни было условие: так у формы
 * ровно одно представление — и в адресе, и в схеме разбора, и в проверке.
 * Что означает каждая позиция, знает toRequestBody:
 *
 *   any                 все значения списка
 *   contains, is        [0] — текст
 *   equals              [0] — "true" или "false"
 *   between             [0] — с, [1] — по
 *   after, before       [0] — граница
 */
export type Filter = { op: FilterOperator; values: string[] };

/**
 * Фильтры по слагу поля.
 *
 * Ключ есть — фильтр показан в подшапке, даже если значений нет.
 * Так «добавить фильтр» и «задать значение» остаются разными шагами,
 * как в референсе: сначала поле появляется в строке, потом заполняется.
 */
export type Filters = Record<string, Filter>;

/**
 * Разбор фильтров из чужого источника: адресной строки и localStorage.
 * Схема одна на оба — форма у них одинаковая, а испорченному значению
 * веры нет ни там, ни там.
 */
export const filtersSchema = z.record(
  z.string(),
  z.object({ op: z.enum(FILTER_OPERATORS), values: z.array(z.string()) }),
);

/** Непонятное значение — просто «фильтров нет», а не сломанный экран. */
export function parseFilters(value: unknown): Filters | undefined {
  const parsed = filtersSchema.safeParse(value);
  return parsed.success ? parsed.data : undefined;
}

export type ItemsQuery = {
  limit: number;
  /** Номер страницы с единицы — как её видит человек. */
  page: number;
  /** Сортировок может быть несколько; порядок в списке — приоритет. */
  sorts?: Sort[] | undefined;
  filters?: Filters | undefined;
  /** Общий поиск по текстовым полям. */
  search?: string | undefined;
};

/**
 * Тело POST /v2/object/get-list/{slug}.
 *
 * Фильтры кладутся плоскими ключами рядом со служебными полями —
 * так устроена ручка. Поэтому поле со слагом `limit` или `search`
 * перезаписало бы служебное: служебные ставятся ПОСЛЕ фильтров,
 * и выигрывают они, а не данные.
 */
export function toRequestBody(query: ItemsQuery): Record<string, unknown> {
  const { limit, page, sorts, filters, search } = query;

  const body: Record<string, unknown> = toConditions(filters ?? {});

  body["limit"] = limit;
  body["offset"] = (Math.max(page, 1) - 1) * limit;

  /*
   * Направление числом — так его ждёт бэкенд (1 — по возрастанию).
   * Несколько сортировок — несколько ключей в одном объекте; приоритет
   * задаётся порядком вставки, поэтому список не пересортировывается.
   */
  const order: Record<string, number> = {};
  for (const sort of sorts ?? []) {
    if (sort.field && !(sort.field in order)) {
      order[sort.field] = sort.direction === "asc" ? 1 : -1;
    }
  }
  if (Object.keys(order).length) body["order"] = order;
  if (search?.trim()) body["search"] = search.trim();

  return body;
}

/**
 * Фильтры → карта условий по слагу, без служебных полей.
 *
 * В этом же виде их хранит и настройка view (attributes.default_filters):
 * старая админка писала туда ровно то, что уходит в тело запроса, и
 * менять формат ради своего было бы обменом «настройки старой админки»
 * на «настройки новой».
 */
export function toConditions(filters: Filters): Record<string, unknown> {
  const conditions: Record<string, unknown> = {};

  for (const [slug, filter] of Object.entries(filters)) {
    const ready = toCondition(filter);
    if (ready !== undefined) conditions[slug] = ready;
  }

  return conditions;
}

/**
 * Обратное преобразование: карта условий → фильтры.
 *
 * Условие не несёт имени операции, поэтому она восстанавливается по
 * форме значения — так же, как её собирал toCondition. Однозначно
 * восстанавливается всё, кроме `contains`: голая строка приходит и от
 * него, и от точного совпадения по числу. Берётся `contains` — он
 * находит надмножество, и человек в чипе видит, что именно ищется.
 */
export function fromConditions(raw: unknown): Filters {
  if (typeof raw !== "object" || raw === null || Array.isArray(raw)) return {};

  const filters: Filters = {};

  for (const [slug, value] of Object.entries(raw as Record<string, unknown>)) {
    const filter = toFilter(value);
    if (filter) filters[slug] = filter;
  }

  return filters;
}

function toFilter(value: unknown): Filter | null {
  if (Array.isArray(value)) {
    const values = value.map(String).filter(Boolean);
    return values.length ? { op: "any", values } : null;
  }

  if (typeof value === "boolean") return { op: "equals", values: [String(value)] };

  if (typeof value === "object" && value !== null) {
    const range = value as Record<string, unknown>;
    const first = range["$in"];

    if (Array.isArray(first)) {
      const exact = first[0];
      return exact === undefined ? null : { op: "is", values: [String(exact)] };
    }

    const gte = range["$gte"];
    const lte = range["$lte"];
    if (gte !== undefined || lte !== undefined) {
      return { op: "between", values: [String(gte ?? ""), String(lte ?? "")] };
    }

    if (range["$gt"] !== undefined) return { op: "after", values: [String(range["$gt"])] };
    if (range["$lt"] !== undefined) return { op: "before", values: [String(range["$lt"])] };

    return null;
  }

  const text = String(value ?? "").trim();
  return text ? { op: "contains", values: [text] } : null;
}

/** undefined — фильтр добавлен, но не заполнен: на сервер он не уходит. */
function toCondition(filter: Filter): unknown {
  const values = filter.values.map((value) => value.trim());
  const first = values[0] ?? "";

  switch (filter.op) {
    case "any": {
      const chosen = values.filter(Boolean);
      return chosen.length ? chosen : undefined;
    }

    case "contains":
      return first || undefined;

    case "is":
      return first ? { $in: [first] } : undefined;

    case "equals":
      // Третьего состояния нет: пока не выбрали, фильтр не заполнен.
      if (first === "true") return true;
      if (first === "false") return false;
      return undefined;

    case "after":
      return first ? { $gt: first } : undefined;

    case "before":
      return first ? { $lt: first } : undefined;

    case "between": {
      const range: Record<string, string> = {};
      if (first) range["$gte"] = first;
      if (values[1]) range["$lte"] = values[1];
      return Object.keys(range).length ? range : undefined;
    }
  }
}

/** Заполнен ли фильтр — для подсветки чипа и счётчика. */
export function isFilterSet(filter: Filter | undefined): boolean {
  return filter !== undefined && toCondition(filter) !== undefined;
}

/**
 * Клик по заголовку колонки: вверх → вниз → без сортировки.
 *
 * Заголовок задаёт сортировку целиком, а не добавляет ещё одну: клик
 * по колонке — это «показать по этому полю», а несколько условий
 * собираются в панели сортировки, где их видно все сразу.
 */
export function nextSorts(current: Sort[], field: string): Sort[] {
  const only = current.length === 1 ? current[0] : undefined;

  if (only?.field !== field) return [{ field, direction: "asc" }];
  if (only.direction === "asc") return [{ field, direction: "desc" }];
  return [];
}

/** «слаг:направление,…» — читаемо в адресной строке и разбирается split'ом. */
export function parseSorts(value: string | undefined): Sort[] {
  if (!value) return [];

  return value
    .split(",")
    .map((part) => {
      const [field, direction] = part.split(":");
      return field ? { field, direction: direction === "desc" ? "desc" : "asc" } : null;
    })
    .filter((sort): sort is Sort => sort !== null);
}

export function formatSorts(sorts: Sort[]): string | undefined {
  const value = sorts
    .filter((sort) => sort.field)
    .map((sort) => `${sort.field}:${sort.direction}`)
    .join(",");

  return value || undefined;
}

/** Сколько фильтров заполнено. Добавленные, но пустые не считаются. */
export function activeFilterCount(filters: Filters | undefined): number {
  return Object.values(filters ?? {}).filter(isFilterSet).length;
}
