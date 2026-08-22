import { expect, test } from "vitest";
import { toUpdateBody, toView } from "./views";

test("имя вкладки собирается по языкам данных, пустые не в счёт", () => {
  const view = toView({
    id: "v",
    type: "TABLE",
    name: " Приемка ",
    attributes: { name_ru: "Приемка", name_en: "", name_uz: "Qabul", label_ru: "чужое" },
  });

  // Пустой name_en не должен перебивать базовое имя, а label_ru — не имя вовсе.
  expect(view.names).toEqual({ ru: "Приемка", uz: "Qabul" });
  expect(view.name).toBe("Приемка");
});

test("view без имени остаётся без имени, а не с пустой строкой в языках", () => {
  const view = toView({ id: "v", type: "BOARD" });

  expect(view.name).toBe("");
  expect(view.names).toEqual({});
});

/*
 * PUT перезаписывает строку целиком, а calendar_*, group_fields и columns
 * бэкенд проставляет ВСЕГДА, даже когда их не прислали. Тело, собранное
 * заново, стёрло бы настройки, которых мы не показываем, — поэтому оно
 * строится поверх исходного ответа.
 */
const stored = toView({
  id: "v",
  type: "TABLE",
  table_slug: "t",
  name: "Приемка",
  columns: ["a", "b"],
  attributes: { name_ru: "Приемка", quick_filters: [], summaries: ["x"] },
  // В нашем ViewDto этих ключей нет — они доезжают через raw.
  ...({ group_fields: ["g"], calendar_from_slug: "from" } as object),
});

test("правка имени не трогает колонки и чужие настройки", () => {
  const body = toUpdateBody({ view: stored, name: " Recieve ", language: "en" });

  expect(body["columns"]).toEqual(["a", "b"]);
  expect(body["group_fields"]).toEqual(["g"]);
  expect(body["calendar_from_slug"]).toBe("from");
  expect(body["name"]).toBe("Recieve");
  expect(body["attributes"]).toEqual({
    name_ru: "Приемка",
    name_en: "Recieve",
    quick_filters: [],
    summaries: ["x"],
  });
});

test("правка колонок не трогает имя", () => {
  const body = toUpdateBody({ view: stored, columns: ["b"] });

  expect(body["columns"]).toEqual(["b"]);
  expect(body["name"]).toBe("Приемка");
  expect(body["attributes"]).toEqual({
    name_ru: "Приемка",
    quick_filters: [],
    summaries: ["x"],
  });
});

/*
 * Формат настроек мы делим со старой админкой: она читает те же
 * attributes, и промах в ключе или флаге у неё выглядит как «настройка
 * не сохранилась».
 */
test("быстрые фильтры уходят с is_checked — по нему их считает старая админка", () => {
  const field = {
    id: "f1",
    slug: "status",
    label: "Статус",
    labels: {},
    type: "STATUS",
    relationId: null,
    options: new Map(),
    multilanguage: false,
    hasColor: false,
    required: false,
    validation: null,
    editable: true,
    attributes: {},
    raw: { id: "f1", slug: "status", type: "STATUS" },
  };

  const body = toUpdateBody({ view: stored, quickFilters: [field] });
  const attributes = body["attributes"] as Record<string, unknown>;

  expect(attributes["quick_filters"]).toEqual([
    { id: "f1", slug: "status", type: "STATUS", is_checked: true },
  ]);
});

test("закреплённые колонки лежат объектом, а снятые действительно снимаются", () => {
  const body = toUpdateBody({ view: stored, fixedColumns: ["f1", "f2"] });
  const attributes = body["attributes"] as Record<string, unknown>;

  expect(attributes["fixedColumns"]).toEqual({ f1: true, f2: true });

  // Ключ со значением false — снятая колонка, а не закреплённая.
  expect(toView({ id: "v", attributes: { fixedColumns: { f1: true, f2: false } } }).fixedColumnIds)
    .toEqual(["f1"]);
});

/*
 * Раскладка вкладками лежит колонкой таблицы, а не в attributes: бэкенд
 * пишет её отдельным `group_fields = $N` при каждом PUT.
 */
test("поле раскладки вкладками читается из group_fields", () => {
  expect(toView({ id: "v", group_fields: ["rel-1"] }).tabGroupId).toBe("rel-1");
  expect(toView({ id: "v" }).tabGroupId).toBe("");
  expect(toView({ id: "v", group_fields: [] }).tabGroupId).toBe("");
});

test("правка раскладки вкладками не трогает attributes и колонки", () => {
  const body = toUpdateBody({ view: stored, tabGroup: "rel-1" });

  expect(body["group_fields"]).toEqual(["rel-1"]);
  expect(body["columns"]).toEqual(["a", "b"]);
  expect(body["attributes"]).toEqual({ name_ru: "Приемка", quick_filters: [], summaries: ["x"] });
});

test("снятие раскладки шлёт пустой список, а не молчит", () => {
  // Молчание вернуло бы прежнее значение из raw: PUT собирается поверх него.
  expect(toUpdateBody({ view: stored, tabGroup: "" })["group_fields"]).toEqual([]);
  expect(toUpdateBody({ view: stored, columns: ["b"] })["group_fields"]).toEqual(["g"]);
});

/*
 * Даты календаря: истина — колонка, но у view, созданных старой
 * админкой, слаги лежат ТОЛЬКО в attributes: её форма создания писала
 * туда, а сохранение — в колонку. Без ступени такой календарь открылся
 * бы пустым экраном «выберите поле даты».
 */
test("поля дат читаются из колонок, а у старых view — из attributes", () => {
  expect(toView({ id: "v", calendar_from_slug: "starts_at", calendar_to_slug: "ends_at" }))
    .toMatchObject({ dateFromSlug: "starts_at", dateToSlug: "ends_at" });

  expect(
    toView({ id: "v", attributes: { calendar_from_slug: "starts_at", calendar_to_slug: "ends_at" } }),
  ).toMatchObject({ dateFromSlug: "starts_at", dateToSlug: "ends_at" });

  // Колонка сильнее: она и есть то, что мы пишем.
  expect(
    toView({
      id: "v",
      calendar_from_slug: "starts_at",
      attributes: { calendar_from_slug: "старое" },
    }).dateFromSlug,
  ).toBe("starts_at");

  expect(toView({ id: "v" }).dateFromSlug).toBe("");
});

test("правка полей дат не трогает остальные настройки", () => {
  const body = toUpdateBody({ view: stored, dateFrom: "starts_at" });

  expect(body["calendar_from_slug"]).toBe("starts_at");
  // Конец не правили — уезжает то, что лежало во view.
  expect(body["calendar_to_slug"]).toBeUndefined();
  expect(body["columns"]).toEqual(["a", "b"]);
  expect(body["attributes"]).toEqual({ name_ru: "Приемка", quick_filters: [], summaries: ["x"] });

  // Снятие поля конца шлёт пустую строку, а не молчит: молчание вернуло
  // бы прежнее значение из raw.
  expect(toUpdateBody({ view: stored, dateTo: "" })["calendar_to_slug"]).toBe("");
});
