import { describe, expect, it, test } from "vitest";
import {
  activeFilterCount,
  formatSorts,
  fromConditions,
  isFilterSet,
  nextSorts,
  parseFilters,
  parseSorts,
  toConditions,
  toRequestBody,
  type Filters,
} from "./query";

test("страница превращается в offset", () => {
  expect(toRequestBody({ limit: 25, page: 1 })).toMatchObject({ limit: 25, offset: 0 });
  expect(toRequestBody({ limit: 25, page: 4 })).toMatchObject({ limit: 25, offset: 75 });
});

test("направление сортировки уходит числом", () => {
  expect(toRequestBody({ limit: 10, page: 1, sorts: [{ field: "title", direction: "asc" }] }).order)
    .toEqual({ title: 1 });
  expect(toRequestBody({ limit: 10, page: 1, sorts: [{ field: "title", direction: "desc" }] }).order)
    .toEqual({ title: -1 });
});

test("несколько сортировок сохраняют приоритет", () => {
  // Приоритет задан порядком ключей в объекте, поэтому список
  // не пересортировывается.
  const order = toRequestBody({
    limit: 10,
    page: 1,
    sorts: [
      { field: "status", direction: "desc" },
      { field: "title", direction: "asc" },
    ],
  }).order;

  expect(Object.keys(order as object)).toEqual(["status", "title"]);
  expect(order).toEqual({ status: -1, title: 1 });
});

test("повтор поля не создаёт второй ключ", () => {
  const order = toRequestBody({
    limit: 10,
    page: 1,
    sorts: [
      { field: "title", direction: "asc" },
      { field: "title", direction: "desc" },
    ],
  }).order;

  expect(order).toEqual({ title: 1 });
});

test("сортировка в адресе читается и пишется одинаково", () => {
  const sorts = parseSorts("status:desc,title:asc");

  expect(sorts).toEqual([
    { field: "status", direction: "desc" },
    { field: "title", direction: "asc" },
  ]);
  expect(formatSorts(sorts)).toBe("status:desc,title:asc");
  expect(formatSorts([])).toBeUndefined();
  expect(parseSorts(undefined)).toEqual([]);
  // Неизвестное направление — по возрастанию, а не ошибка.
  expect(parseSorts("title:мусор")).toEqual([{ field: "title", direction: "asc" }]);
});

/*
 * Формы условий сверены с живым бэкендом: то, чего здесь нет, отвечает
 * 500 — $ne, $nin и $exists ручка не понимает.
 */
test("каждое условие превращается в свою форму", () => {
  const body = toRequestBody({
    limit: 10,
    page: 1,
    filters: {
      status: { op: "any", values: ["todo", "done"] },
      title: { op: "contains", values: [" текст "] },
      code: { op: "is", values: ["ABC"] },
      active: { op: "equals", values: ["false"] },
      created: { op: "between", values: ["2025-11-01", "2025-11-30"] },
      seen: { op: "after", values: ["2025-11-05"] },
      due: { op: "before", values: ["2025-12-01"] },
    },
  });

  expect(body["status"]).toEqual(["todo", "done"]);
  expect(body["title"]).toBe("текст");
  expect(body["code"]).toEqual({ $in: ["ABC"] });
  expect(body["active"]).toBe(false);
  expect(body["created"]).toEqual({ $gte: "2025-11-01", $lte: "2025-11-30" });
  expect(body["seen"]).toEqual({ $gt: "2025-11-05" });
  expect(body["due"]).toEqual({ $lt: "2025-12-01" });
});

test("«содержит» и «равно» дают разные запросы", () => {
  // На сервере это LIKE против точного совпадения: "asd" находит
  // 6 строк вхождением и 0 — точным сравнением.
  const contains = toRequestBody({ limit: 1, page: 1, filters: { t: { op: "contains", values: ["asd"] } } });
  const is = toRequestBody({ limit: 1, page: 1, filters: { t: { op: "is", values: ["asd"] } } });

  expect(contains["t"]).toBe("asd");
  expect(is["t"]).toEqual({ $in: ["asd"] });
});

test("односторонний диапазон уходит одной границей", () => {
  expect(toRequestBody({ limit: 10, page: 1, filters: { d: { op: "between", values: ["2025-11-01"] } } }).d)
    .toEqual({ $gte: "2025-11-01" });
});

test("добавленный, но не заполненный фильтр на сервер не уходит", () => {
  // Ключ есть — чип показан в подшапке; значений нет — запрос без него.
  const body = toRequestBody({
    limit: 10,
    page: 1,
    filters: {
      status: { op: "any", values: [] },
      title: { op: "contains", values: ["  "] },
      created: { op: "between", values: [] },
      active: { op: "equals", values: [] },
      kind: { op: "any", values: ["a"] },
    },
  });

  expect(body).not.toHaveProperty("status");
  expect(body).not.toHaveProperty("title");
  expect(body).not.toHaveProperty("created");
  expect(body).not.toHaveProperty("active");
  expect(body["kind"]).toEqual(["a"]);
});

test("«нет» — это заполненный фильтр, а не пустой", () => {
  expect(isFilterSet({ op: "equals", values: ["false"] })).toBe(true);
  expect(isFilterSet({ op: "equals", values: [] })).toBe(false);
  expect(isFilterSet(undefined)).toBe(false);
});

test("поле со служебным слагом не перебивает служебное значение", () => {
  // В таблице пользователя может быть поле с именем limit или search.
  // Фильтры кладутся плоскими ключами, поэтому порядок здесь — не стиль.
  const body = toRequestBody({
    limit: 25,
    page: 2,
    search: "текст",
    filters: {
      limit: { op: "any", values: ["999"] },
      offset: { op: "any", values: ["999"] },
      search: { op: "contains", values: ["чужое"] },
      order: { op: "any", values: ["чужое"] },
    },
    sorts: [{ field: "title", direction: "asc" }],
  });

  expect(body["limit"]).toBe(25);
  expect(body["offset"]).toBe(25);
  expect(body["search"]).toBe("текст");
  expect(body["order"]).toEqual({ title: 1 });
});

test("пустой поиск не уходит, пробелы обрезаются", () => {
  expect(toRequestBody({ limit: 10, page: 1, search: "   " })).not.toHaveProperty("search");
  expect(toRequestBody({ limit: 10, page: 1, search: " да " }).search).toBe("да");
});

test("клик по заголовку: вверх → вниз → без сортировки", () => {
  const first = nextSorts([], "title");
  expect(first).toEqual([{ field: "title", direction: "asc" }]);

  const second = nextSorts(first, "title");
  expect(second).toEqual([{ field: "title", direction: "desc" }]);

  expect(nextSorts(second, "title")).toEqual([]);
});

test("клик по заголовку заменяет всю сортировку, а не дополняет", () => {
  // Иначе колонки накапливались бы молча: видимого списка условий
  // у заголовка нет, и человек не понял бы, почему порядок не меняется.
  const many = [
    { field: "a", direction: "asc" as const },
    { field: "b", direction: "asc" as const },
  ];

  expect(nextSorts(many, "b")).toEqual([{ field: "b", direction: "asc" }]);
});

test("счётчик считает только заполненные фильтры", () => {
  expect(
    activeFilterCount({
      a: { op: "any", values: ["1"] },
      b: { op: "any", values: [] },
      c: { op: "contains", values: ["текст"] },
      d: { op: "between", values: [] },
      e: { op: "equals", values: ["true"] },
    }),
  ).toBe(3);
  expect(activeFilterCount(undefined)).toBe(0);
});

/*
 * Отбор приходит из адреса и из localStorage — обоим верить нельзя:
 * чужое условие валит запрос пятисоткой, а испорченный ключ — экран.
 */
describe("parseFilters", () => {
  it("пропускает годный отбор", () => {
    const value = { name: { op: "contains", values: ["мясо"] } };
    expect(parseFilters(value)).toEqual(value);
  });

  it("несуществующее условие и мусор — как будто отбора нет", () => {
    expect(parseFilters({ name: { op: "starts_with", values: [] } })).toBeUndefined();
    expect(parseFilters({ name: { op: "contains", values: [42] } })).toBeUndefined();
    expect(parseFilters("что угодно")).toBeUndefined();
    expect(parseFilters(undefined)).toBeUndefined();
  });
});

describe("отбор по умолчанию: карта условий ↔ фильтры", () => {
  const filters: Filters = {
    status: { op: "any", values: ["todo", "done"] },
    title: { op: "contains", values: ["акт"] },
    code: { op: "is", values: ["A-1"] },
    paid: { op: "equals", values: ["true"] },
    created: { op: "between", values: ["2024-01-01", "2024-12-31"] },
    updated: { op: "after", values: ["2024-06-01"] },
    closed: { op: "before", values: ["2025-01-01"] },
  };

  it("переживает круг «сохранили — прочитали»", () => {
    expect(fromConditions(toConditions(filters))).toEqual(filters);
  });

  it("незаполненный фильтр в настройку не попадает", () => {
    expect(toConditions({ title: { op: "contains", values: [] } })).toEqual({});
  });

  it("мусор вместо настройки — это «отбора нет», а не сломанный экран", () => {
    expect(fromConditions(undefined)).toEqual({});
    expect(fromConditions("нет")).toEqual({});
    expect(fromConditions([1, 2])).toEqual({});
  });
});
