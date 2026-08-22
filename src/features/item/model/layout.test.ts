import { expect, test } from "vitest";
import type { Field } from "@/features/table";
import {
  addSection,
  applyRights,
  fieldOrder,
  fieldRights,
  headingSlug,
  hiddenFields,
  itemTitle,
  moveField,
  orderColumns,
  removeSection,
  renameSection,
  sections,
  setHeading,
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

test("поле переезжает в секцию цели, а соседи остаются на местах", () => {
  // Раньше размеры секций сохранялись: «d», брошенное в первую секцию,
  // выталкивало оттуда «b» во вторую — то есть один перенос двигал два
  // поля. Теперь переезжает ровно то, что тащили.
  const tab = moveField(layout(), "d", "a", false).tabs?.[1];
  expect(tab?.sections?.map((section) => section.fields?.map((field) => field.slug))).toEqual([
    ["d", "a", "b"],
    ["c"],
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

test("права роли на поля читаются из раскладки: запрет строгий, разрешение по умолчанию", () => {
  const rights = fieldRights({
    tabs: [
      {
        type: "section",
        sections: [
          {
            fields: [
              { slug: "secret", attributes: { field_permission: { view_permission: false } } },
              { slug: "locked", attributes: { field_permission: { edit_permission: false } } },
              { slug: "open", attributes: { field_permission: { view_permission: true } } },
              // Ни блока прав, ни записи в field_permission — поле обычное.
              { slug: "plain" },
            ],
          },
        ],
      },
    ],
  });

  expect([...rights.hidden]).toEqual(["secret"]);
  expect([...rights.readonly]).toEqual(["locked"]);

  const field = (slug: string): Field => ({ slug, editable: true }) as Field;
  const columns = applyRights([field("secret"), field("locked"), field("plain")], rights);

  expect(columns.map((item) => item.slug)).toEqual(["locked", "plain"]);
  expect(columns.map((item) => item.editable)).toEqual([false, true]);
});

test("заголовок записи — только скалярное значение", () => {
  expect(itemTitle({ name: "Заказ 12" }, "name")).toBe("Заказ 12");
  expect(itemTitle({ number: 12 }, "number")).toBe("12");
  // Поле-заголовок сменило тип: объект в шапке — это `[object Object]`.
  expect(itemTitle({ name: { ru: "Заказ" } }, "name")).toBe("");
  expect(itemTitle({ name: null }, "name")).toBe("");
  expect(itemTitle(undefined, "name")).toBe("");
  // Заголовок не назначен вовсе.
  expect(itemTitle({ name: "Заказ 12" }, "")).toBe("");
});

/*
 * Секции карточки правятся: их заводят, переименовывают и удаляют,
 * а поля переносят между ними мышью. Раньше секции только читались.
 */
const twoSections = (): Layout => ({
  tabs: [
    {
      type: "section",
      sections: [
        { label: "Основное", fields: [{ slug: "name" }, { slug: "email" }] },
        { label: "Служебное", fields: [{ slug: "created_at" }] },
      ],
    },
  ],
});

test("поле переезжает В СЕКЦИЮ цели, а не просто на её место", () => {
  // Раньше размеры секций сохранялись, и поле, брошенное во вторую
  // секцию, выталкивало оттуда соседа обратно в первую.
  const next = moveField(twoSections(), "name", "created_at", true);

  expect(sections(next)).toEqual([
    { label: "Основное", slugs: ["email"] },
    { label: "Служебное", slugs: ["created_at", "name"] },
  ]);
});

test("секция заводится пустой: поля в неё переносят мышью", () => {
  expect(sections(addSection(twoSections(), "Контакты"))).toEqual([
    { label: "Основное", slugs: ["name", "email"] },
    { label: "Служебное", slugs: ["created_at"] },
    { label: "Контакты", slugs: [] },
  ]);
});

test("переименование не трогает поля", () => {
  const next = renameSection(twoSections(), 1, "Система");

  expect(sections(next)[1]).toEqual({ label: "Система", slugs: ["created_at"] });
});

test("имя секции переживает перезапрос: оно уходит и в attributes", () => {
  /*
   * Колонку section.label PUT пишет, а ни один GET не возвращает
   * (layout.go:1367, GetSections) — переименованная секция приезжала
   * назад безымянной. Имя дублируется в attributes, который ездит
   * в обе стороны, и читается сначала оттуда.
   */
  const tab = (renameSection(twoSections(), 1, "Система").tabs ?? [])[0];
  const section = (tab?.sections ?? [])[1] as { label?: string; attributes?: unknown };

  expect(section.attributes).toEqual({ label: "Система" });
  // Колонка тоже заполняется: её читает старая админка.
  expect(section.label).toBe("Система");

  // Как это вернётся с сервера: колонка пустая, attributes на месте.
  const asServerReturns: Layout = {
    tabs: [{ type: "section", sections: [{ attributes: { label: "Система" }, fields: [] }] }],
  };

  expect(sections(asServerReturns)).toEqual([{ label: "Система", slugs: [] }]);
});

test("удаление секции не теряет её поля", () => {
  // Иначе поля исчезли бы из карточки целиком, и вернуть их можно было бы
  // только правкой раскладки руками.
  expect(sections(removeSection(twoSections(), 1))).toEqual([
    { label: "Основное", slugs: ["name", "email", "created_at"] },
  ]);

  // Первая секция отдаёт поля вниз: выше неё ничего нет.
  expect(sections(removeSection(twoSections(), 0))).toEqual([
    { label: "Служебное", slugs: ["created_at", "name", "email"] },
  ]);
});

test("последнюю секцию удалить нельзя: полям некуда деться", () => {
  const single: Layout = {
    tabs: [{ type: "section", sections: [{ label: "", fields: [{ slug: "name" }] }] }],
  };

  expect(removeSection(single, 0)).toBe(single);
});
