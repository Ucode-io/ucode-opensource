import { expect, test } from "vitest";
import { diffEntry, unwrapEntry } from "./activity";

test("конверт data снимается, объект раскладывается по строкам", () => {
  expect(unwrapEntry('{"data":{"name":"Склад"}}')).toBe('{\n  "name": "Склад"\n}');
});

test("объект без конверта читается так же", () => {
  expect(unwrapEntry('{"name":"Склад"}')).toBe('{\n  "name": "Склад"\n}');
});

/* Шлюз подставляет пустую карту вместо отсутствующего значения. */
test("пустая карта — это отсутствие значения, а не пустой объект", () => {
  expect(unwrapEntry('{"data":{}}')).toBe("");
  expect(unwrapEntry("")).toBe("");
});

test("текст ошибки не JSON — показывается как есть", () => {
  expect(unwrapEntry("rpc error: code = Unavailable")).toBe("rpc error: code = Unavailable");
  expect(unwrapEntry('{"data":"not found"}')).toBe("not found");
});

/*
 * Настоящая запись UPDATE VIEW: два десятка идентификаторов колонок,
 * из которых меняется один. Ради этого случая разбор и писался — всё
 * остальное здесь про то, чтобы он не наврал на соседних формах данных.
 */
const columns = Array.from({ length: 25 }, (_, index) => `c-${index}`);
const view = (extra: Record<string, unknown>) =>
  JSON.stringify({ data: { id: "v-1", type: "CHART", order: 10, columns, ...extra } });

test("добавили одну колонку — одна строка, а не два полотна", () => {
  expect(diffEntry(view({}), view({ columns: [...columns, "c-25"] }))).toEqual([
    { path: "columns", added: ["c-25"], removed: [] },
  ]);
});

test("убрали одну колонку", () => {
  expect(diffEntry(view({}), view({ columns: columns.filter((id) => id !== "c-7") }))).toEqual([
    { path: "columns", added: [], removed: ["c-7"] },
  ]);
});

/* Тот же набор в другом порядке: сравнение по индексам объявило бы
   изменившимися все двадцать пять. */
test("перестановка — это перестановка, а не двадцать пять правок", () => {
  expect(diffEntry(view({}), view({ columns: [...columns].reverse() }))).toEqual([
    { path: "columns", added: [], removed: [] },
  ]);
});

test("вложенное поле находится по пути, соседние молчат", () => {
  const before = '{"data":{"attributes":{"default_limit":10,"name_ru":"Свод"}}}';
  const after = '{"data":{"attributes":{"default_limit":20,"name_ru":"Свод"}}}';

  expect(diffEntry(before, after)).toEqual([
    { path: "attributes.default_limit", before: "10", after: "20" },
  ]);
});

test("правка одного элемента списка разбирается по полю", () => {
  const before = '{"data":{"filters":[{"field":"status","value":"new"}]}}';
  const after = '{"data":{"filters":[{"field":"status","value":"done"}]}}';

  expect(diffEntry(before, after)).toEqual([
    { path: "filters[0].value", before: "new", after: "done" },
  ]);
});

/* Заменили не один элемент, а несколько — разбирать попарно уже нечего:
   какой чему соответствует, знает только тот, кто правил. */
test("замена нескольких элементов — списками", () => {
  const before = '{"data":{"filters":[{"f":"a"},{"f":"b"}]}}';
  const after = '{"data":{"filters":[{"f":"c"},{"f":"d"}]}}';

  expect(diffEntry(before, after)).toEqual([
    {
      path: "filters",
      added: ['{"f":"c"}', '{"f":"d"}'],
      removed: ['{"f":"a"}', '{"f":"b"}'],
    },
  ]);
});

test("строки в списке показываются без кавычек", () => {
  expect(diffEntry('{"data":{"tags":["a"]}}', '{"data":{"tags":["a","b"]}}')).toEqual([
    { path: "tags", added: ["b"], removed: [] },
  ]);
});

test("появление и пропажа поля видны", () => {
  expect(diffEntry('{"data":{}}', '{"data":{"label":"Новое"}}')).toEqual([
    { path: "label", before: "", after: "Новое" },
  ]);
  expect(diffEntry('{"data":{"label":"Было"}}', '{"data":{}}')).toEqual([
    { path: "label", before: "Было", after: "" },
  ]);
});

/* `null` — это записанное значение, а пусто — отсутствие поля. Разница
   видна: в базе это разные вещи. */
test("null отличается от отсутствия поля", () => {
  expect(diffEntry('{"data":{"x":null}}', '{"data":{}}')).toEqual([
    { path: "x", before: "null", after: "" },
  ]);
});

test("смена типа не ломает разбор", () => {
  expect(diffEntry('{"data":{"columns":["a"]}}', '{"data":{"columns":"a"}}')).toEqual([
    { path: "columns", before: '[\n  "a"\n]', after: "a" },
  ]);
});

test("порядок ключей в объекте — не различие", () => {
  expect(diffEntry('{"data":{"a":1,"b":2}}', '{"data":{"b":2,"a":1}}')).toEqual([]);
});

/* Глубже предела путь не читается, и вместо него показывается ветка. */
test("вложенность ограничена", () => {
  const deep = (leaf: number) =>
    JSON.stringify({ data: { a: { b: { c: { d: { e: { f: { g: leaf } } } } } } } });

  const changes = diffEntry(deep(1), deep(2));

  expect(changes).toHaveLength(1);
  expect(changes[0]?.path).toBe("a.b.c.d.e.f");
});

test("сравнивать нечего — пусто, а не выдуманное различие", () => {
  expect(diffEntry(view({}), view({}))).toEqual([]);
  expect(diffEntry("", "")).toEqual([]);
  // Текст ошибки вместо объекта: разбирать по полям нечего.
  expect(diffEntry("rpc error", "rpc error: code = Unavailable")).toEqual([]);
});
