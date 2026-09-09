import { expect, test } from "vitest";
import type { Relation } from "@/features/table";
import { relationTabs, tabbableRelations } from "./relation-tabs";
import type { View } from "./types";

const relation = (patch: Partial<Relation> = {}): Relation =>
  ({
    id: "r1",
    type: "Many2One",
    toSlug: "order",
    toLabel: "Заказы",
    toLabels: {},
    title: "",
    fieldFrom: "",
    direction: "incoming",
    linkField: "customer_id",
    viewFieldSlugs: [],
    viewFieldIds: [],
    raw: {},
    ...patch,
  }) as Relation;

const view = (patch: Partial<View> = {}): View =>
  ({
    id: "v1",
    type: "TABLE",
    tableSlug: "customer",
    name: "",
    names: {},
    order: 0,
    defaultLimit: null,
    isRelationView: true,
    relationTableSlug: "order",
    relationId: "",
    tableLabel: "",
    quickFilterIds: [],
    columnIds: [],
    fixedColumnIds: [],
    defaultFilters: {},
    navigate: { url: "", params: [] },
    objectUrl: { url: "", params: [] },
    pdfUrl: "",
  infiniteScroll: false,
    raw: {},
    ...patch,
  }) as View;

test("вкладками становятся только relation view пункта меню", () => {
  const views = [
    view({ id: "table", isRelationView: false, relationTableSlug: "" }),
    view({ id: "tab" }),
  ];

  expect(relationTabs(views, [relation()], "ru").map((tab) => tab.id)).toEqual(["tab"]);
});

test("связь ищется по relation_id, а при его отсутствии — по слагу таблицы", () => {
  const relations = [
    relation({ id: "r1", linkField: "sender_id" }),
    relation({ id: "r2", linkField: "receiver_id" }),
  ];

  // Заведённые старой админкой view знают только слаг чужой таблицы:
  // из двух связей на неё выбрать нельзя, берётся первая.
  expect(relationTabs([view()], relations, "ru")[0]?.fieldSlug).toBe("sender_id");

  // Свой view знает связь: колонка-ссылка находится однозначно.
  expect(relationTabs([view({ relationId: "r2" })], relations, "ru")[0]?.fieldSlug).toBe(
    "receiver_id",
  );
});

test("имя вкладки: своё, потом подпись чужой таблицы, потом имя связи", () => {
  const named = view({ names: { ru: "Склад получатель" } });

  expect(relationTabs([named], [relation()], "ru")[0]?.label).toBe("Склад получатель");
  expect(relationTabs([view({ tableLabel: "Заказы" })], [relation()], "ru")[0]?.label).toBe(
    "Заказы",
  );
  expect(
    relationTabs([view()], [relation({ title: "Склад получатель" })], "ru")[0]?.label,
  ).toBe("Склад получатель");
});

test("вкладка без связи пропускается, а не показывает чужую таблицу целиком", () => {
  // Связь удалили из конструктора — отбирать строки нечем.
  expect(relationTabs([view({ relationTableSlug: "gone" })], [relation()], "ru")).toEqual([]);
  expect(relationTabs([view()], [relation({ linkField: "" })], "ru")).toEqual([]);
});

test("создавать связанные строки можно только по входящей связи", () => {
  expect(relationTabs([view()], [relation()], "ru")[0]?.canCreate).toBe(true);
  // У обратной стороны привязка — это правка НАШЕЙ строки.
  expect(relationTabs([view()], [relation({ direction: "outgoing" })], "ru")[0]?.canCreate).toBe(
    false,
  );
});

test("во вкладку годится связь с одной колонкой-ссылкой", () => {
  const list = [
    relation({ id: "a", type: "Many2One" }),
    relation({ id: "b", type: "Recursive" }),
    // Массив идентификаторов вместо одного — отбор по такой колонке
    // вернёт не то.
    relation({ id: "c", type: "Many2Many", linkField: "order_ids" }),
    relation({ id: "d", type: "Many2Dynamic" }),
    relation({ id: "e", linkField: "" }),
  ];

  expect(tabbableRelations(list).map((item) => item.id)).toEqual(["a", "b"]);
});
