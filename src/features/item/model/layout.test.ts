import { expect, test } from "vitest";
import type { Field } from "@/features/table";
import {
  addRelationTab,
  fieldOrder,
  headingSlug,
  hiddenFields,
  moveField,
  relationTabs,
  removeTab,
  orderColumns,
  sections,
  setHeading,
  setTabColumns,
  type Layout,
} from "./layout";

/** Две секции по два поля — этого хватает, чтобы проверить перенос между ними. */
const layout = (): Layout => ({
  id: "l1",
  tabs: [
    { type: "relation" },
    {
      type: "section",
      sections: [
        { fields: [{ slug: "a" }, { slug: "b" }] },
        { fields: [{ slug: "c" }, { slug: "d" }] },
      ],
    },
  ],
});

const order = (next: Layout) => fieldOrder(next).join("");

test("порядок карточки — это порядок полей во всех секциях подряд", () => {
  expect(order(layout())).toBe("abcd");
});

test("поле встаёт до цели и после неё", () => {
  expect(order(moveField(layout(), "d", "b", false))).toBe("adbc");
  expect(order(moveField(layout(), "d", "b", true))).toBe("abdc");
});

test("движение вниз считается по списку БЕЗ переставляемого поля", () => {
  // Наивный splice по исходному индексу цели вставил бы «a» перед «c»:
  // после удаления «a» индексы всех, кто был правее, уже сдвинулись.
  expect(order(moveField(layout(), "a", "c", true))).toBe("bcad");
});

test("секции сохраняют свои размеры — поле переезжает в соседнюю", () => {
  const tab = moveField(layout(), "d", "a", false).tabs?.[1];
  expect(tab?.sections?.map((section) => section.fields?.map((field) => field.slug))).toEqual([
    ["d", "a"],
    ["b", "c"],
  ]);
});

test("бросок на себя и на поле вне раскладки ничего не меняет", () => {
  const before = layout();
  expect(moveField(before, "b", "b", true)).toBe(before);
  expect(moveField(before, "b", "zzz", true)).toBe(before);
  expect(moveField(before, "zzz", "b", true)).toBe(before);
});

test("колонки выстраиваются по раскладке, незнакомые — в конец", () => {
  const columns = ["c", "zzz", "a"].map((slug) => ({ slug }) as Field);
  expect(orderColumns(columns, ["a", "b", "c"]).map((field) => field.slug)).toEqual([
    "a",
    "c",
    "zzz",
  ]);
});

test("спрятано только то, что спрятали явно", () => {
  const next: Layout = {
    tabs: [
      {
        type: "section",
        sections: [
          {
            fields: [
              { slug: "a", attributes: { field_hide_layout: true } },
              { slug: "b", attributes: { field_hide_layout: false } },
              // Ключа нет вовсе — поле показывается. Новое поле бэкенд
              // дописывает в раскладку без этого ключа.
              { slug: "c" },
            ],
          },
        ],
      },
    ],
  };

  expect(hiddenFields(next)).toEqual(["a"]);
});

test("секции отдаются с именами и слагами", () => {
  const next: Layout = {
    tabs: [
      {
        type: "section",
        sections: [
          { label: " Основное ", fields: [{ slug: "a" }] },
          { fields: [{ slug: "b" }] },
        ],
      },
    ],
  };

  expect(sections(next)).toEqual([
    { label: "Основное", slugs: ["a"] },
    { label: "", slugs: ["b"] },
  ]);
});

test("заголовок мультиязычной карточки читается на своём языке", () => {
  const next: Layout = {
    tabs: [{ type: "section", attributes: { layout_heading: { en: "title_en", cyr: "title_cyr" } } }],
  };

  expect(headingSlug(next, "cyr")).toBe("title_cyr");
  // Языка нет в карте — берём любой заданный, а не пустоту: карточка
  // без заголовка выглядит сломанной, а не «на другом языке».
  expect(headingSlug(next, "ru")).toBe("title_en");
});

test("обычный заголовок — просто слаг, и он же записывается", () => {
  const next: Layout = { tabs: [{ type: "section", attributes: { layout_heading: "name" } }] };

  expect(headingSlug(next, "en")).toBe("name");
  expect(headingSlug(setHeading(next, "code", null), "en")).toBe("code");

  // У мультиязычного поля пишется карта по всем языкам сразу: иначе
  // переключение языка обнуляло бы заголовок.
  const multi = setHeading(next, "title_en", { en: "title_en", cyr: "title_cyr" });
  expect(headingSlug(multi, "cyr")).toBe("title_cyr");
});

/*
 * Вкладки связей: источник — раскладка, а не список view. Здесь есть
 * имя колонки-ссылки и права роли, которых в списке view нет.
 */
const withTabs = (tabs: unknown[]): Layout => ({ tabs } as Layout);

test("вкладка связи собирается из раскладки, вместе с колонкой-ссылкой", () => {
  const next = withTabs([
    { type: "section", sections: [] },
    {
      id: "t1",
      label: "Заказы",
      type: "relation",
      relation: {
        relation_table_slug: "order",
        relation_field_slug: "customer_id",
        columns: ["c1", "c2"],
      },
    },
  ]);

  expect(relationTabs(next)).toEqual([
    {
      id: "t1",
      relationId: "",
      label: "Заказы",
      tableSlug: "order",
      // Не собранное `<родительский слаг>_id`, а настоящее имя колонки:
      // у второй связи на ту же таблицу оно другое.
      fieldSlug: "customer_id",
      columnIds: ["c1", "c2"],
      // Права не настраивали — значит можно.
      canCreate: true,
    },
  ]);
});

/*
 * Колонки-ссылки в ответе layout нет — приезжают только стороны связи.
 * Имя выводится по правилу создания: колонка в table_from называется
 * `<слаг table_to>_id`. Пока этого не делали, вкладки отсеивались
 * фильтром целиком и карточка со связями выглядела карточкой без связей.
 */
test("колонка-ссылка выводится из сторон связи, когда её имени нет", () => {
  const next = withTabs([
    {
      id: "t1",
      label: "Дети",
      type: "relation",
      relation: {
        relation_table_slug: "night_child",
        table_from: { slug: "night_child" },
        table_to: { slug: "test_shmest1" },
      },
    },
  ]);

  expect(relationTabs(next)[0]?.fieldSlug).toBe("test_shmest1_id");
});

test("наша сторона — та, которая не relation_table_slug", () => {
  const next = withTabs([
    {
      id: "t1",
      type: "relation",
      // Здесь чужая таблица — table_to, значит наша — table_from.
      relation: {
        relation_table_slug: "orders",
        table_from: { slug: "clients" },
        table_to: { slug: "orders" },
      },
    },
  ]);

  expect(relationTabs(next)[0]?.fieldSlug).toBe("clients_id");
});

test("запрет на просмотр прячет вкладку, отсутствие прав — нет", () => {
  const tab = (permission: unknown) => ({
    id: "t",
    label: "Заказы",
    type: "relation",
    relation: { relation_table_slug: "order", relation_field_slug: "customer_id", permission },
  });

  expect(relationTabs(withTabs([tab({ view_permission: false })]))).toEqual([]);
  expect(relationTabs(withTabs([tab({ view_permission: true })])).length).toBe(1);
  // Блок permission приходит не отовсюду: его отсутствие — «не настраивали»,
  // а не «нельзя». Иначе вкладки исчезли бы у всех, кто прав не трогал.
  expect(relationTabs(withTabs([tab(undefined)])).length).toBe(1);

  expect(relationTabs(withTabs([tab({ create_permission: false })]))[0]?.canCreate).toBe(false);
});

test("вкладка без колонки-ссылки пропускается, а не показывает всю таблицу", () => {
  const next = withTabs([
    { id: "a", type: "relation", relation: { relation_table_slug: "order" } },
    { id: "b", type: "relation", relation: { relation_field_slug: "customer_id" } },
  ]);

  expect(relationTabs(next)).toEqual([]);
});

test("колонки вкладки правятся только у своей вкладки", () => {
  const layout = {
    tabs: [
      { id: "s", type: "section", sections: [] },
      { id: "t1", relation: { relation_table_slug: "orders", columns: ["a"] } },
      { id: "t2", relation: { relation_table_slug: "clients", columns: ["b"] } },
    ],
  };

  const next = setTabColumns(layout, "t1", ["a", "c"]);

  expect(next.tabs?.[1]?.relation?.columns).toEqual(["a", "c"]);
  // Соседняя вкладка не трогается: PUT перезаписывает раскладку целиком.
  expect(next.tabs?.[2]?.relation?.columns).toEqual(["b"]);
  expect(next.tabs?.[0]).toBe(layout.tabs[0]);
});

test("вкладка помнит свою связь: по ней её заводят и по ней же не заводят второй раз", () => {
  const next = withTabs([
    {
      id: "t1",
      label: "Дети",
      type: "relation",
      relation_id: "r1",
      relation: { relation_table_slug: "night_child", table_from: { slug: "night_child" }, table_to: { slug: "orders" } },
    },
  ]);

  expect(relationTabs(next)[0]?.relationId).toBe("r1");
});

test("вкладка добавляется списком, а не в обход: PUT пишет tabs целиком", () => {
  const layout = withTabs([{ id: "s", type: "section", sections: [] }]);
  const next = addRelationTab(layout, { id: "new", label: "Заказы", relationId: "r9" });

  expect(next.tabs?.length).toBe(2);
  expect(next.tabs?.[1]).toMatchObject({ id: "new", type: "relation", relation_id: "r9" });
  expect(removeTab(next, "new").tabs?.length).toBe(1);
});
