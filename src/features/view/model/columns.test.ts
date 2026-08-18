import { expect, test } from "vitest";
import type { Field } from "@/features/table";
import { moveBefore, pickView, resolveColumns, tabViews } from "./columns";
import { EMPTY_URL_TEMPLATE } from "./url-template";
import type { View } from "./types";

const field = (id: string, over: Partial<Field> = {}): Field => ({
  id,
  slug: id,
  label: id,
  labels: {},
  type: "SINGLE_LINE",
  relationId: null,
  multilanguage: false,
  required: false,
  validation: null,
  editable: true,
  raw: {},
  options: new Map(),
  hasColor: false,
  attributes: {},
  ...over,
});

const view = (columnIds: string[]): View => ({
  id: "v",
  type: "TABLE",
  tableSlug: "t",
  name: "",
  names: {},
  order: 0,
  defaultLimit: null,
  isRelationView: false,
  relationTableSlug: "",
  relationId: "",
  tableLabel: "",
  quickFilterIds: [],
  columnIds,
  fixedColumnIds: [],
  defaultFilters: {},
  navigate: EMPTY_URL_TEMPLATE,
  objectUrl: EMPTY_URL_TEMPLATE,
  pdfUrl: "",
  infiniteScroll: false,
  raw: { columns: columnIds },
});

test("порядок колонок берётся из view, а не из схемы", () => {
  const fields = [field("a"), field("b"), field("c")];

  expect(resolveColumns(view(["c", "a"]), fields).map((f) => f.slug)).toEqual(["c", "a"]);
});

test("колонка-связь ищется по id связи, а не по id поля", () => {
  // Именно на этом таблица со связями теряет колонки: во view лежит
  // relation_field, а не id поля.
  const lookup = field("f1", { slug: "author_id", type: "LOOKUP", relationId: "rel1" });

  expect(resolveColumns(view(["rel1"]), [lookup]).map((f) => f.slug)).toEqual(["author_id"]);
});

test("поле, перечисленное дважды, даёт одну колонку", () => {
  // В columns у связи лежат оба ключа — id поля и id связи. Дубль
  // означал бы два одинаковых React-ключа и осиротевшие ячейки в DOM.
  const lookup = field("f1", { slug: "author_id", type: "LOOKUP", relationId: "rel1" });

  expect(resolveColumns(view(["rel1", "f1"]), [lookup]).map((f) => f.slug)).toEqual(["author_id"]);
  expect(resolveColumns(view(["a", "a"]), [field("a")]).map((f) => f.slug)).toEqual(["a"]);
});

test("удалённое поле пропускается, а не рисуется пустой колонкой", () => {
  expect(resolveColumns(view(["a", "ghost"]), [field("a")]).map((f) => f.slug)).toEqual(["a"]);
});

test("без view колонок нет", () => {
  expect(resolveColumns(undefined, [field("a")])).toEqual([]);
});

test("SECTION и view связей вкладками не показываются", () => {
  // SECTION — раскладка формы в Drawer, а не отдельный экран.
  const section: View = { ...view([]), id: "s", type: "SECTION" };
  const relation: View = { ...view([]), id: "r", type: "TABLE", isRelationView: true };
  const table: View = { ...view([]), id: "t", type: "TABLE" };

  expect(tabViews([section, relation, table]).map((v) => v.id)).toEqual(["t"]);
});

test("вкладки идут в порядке, заданном админом", () => {
  const first: View = { ...view([]), id: "a", type: "BOARD", order: 2 };
  const second: View = { ...view([]), id: "b", type: "GRID", order: 1 };

  expect(tabViews([first, second]).map((v) => v.id)).toEqual(["b", "a"]);
});

test("ссылка на удалённый view открывает первую вкладку, а не пустоту", () => {
  const table: View = { ...view([]), id: "t", type: "TABLE" };
  const board: View = { ...view([]), id: "b", type: "BOARD", order: 1 };

  expect(pickView([table, board], "b")?.id).toBe("b");
  expect(pickView([table, board], "ghost")?.id).toBe("t");
  expect(pickView([], "t")).toBeUndefined();
});

test("перетащенная колонка встаёт перед целью — и сверху вниз, и снизу вверх", () => {
  expect(moveBefore(["a", "b", "c", "d"], "a", "c")).toEqual(["b", "a", "c", "d"]);
  expect(moveBefore(["a", "b", "c", "d"], "d", "b")).toEqual(["a", "d", "b", "c"]);
});

test("бросок на себя и мимо списка порядок не меняют", () => {
  expect(moveBefore(["a", "b"], "a", "a")).toEqual(["a", "b"]);
  expect(moveBefore(["a", "b"], "a", "z")).toEqual(["a", "b"]);
});
