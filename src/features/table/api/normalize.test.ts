import { expect, test } from "vitest";
import { localized } from "../model/types";
import { toField, toRelation } from "./normalize";

/*
 * Формы вариантов взяты с живого бэкенда. Два типа хранятся по-разному,
 * и старый ucode искал вариант по option.value для обоих — поэтому
 * у MULTISELECT не находил ни разу.
 */

test("MULTISELECT: ключ поиска — slug, а не value", () => {
  const field = toField({
    id: "1",
    slug: "dropdown_field",
    type: "MULTISELECT",
    attributes: {
      has_color: true,
      options: [
        {
          slug: "Second",
          value: "Секонд67",
          label: "Секонд67",
          label_cyr: "Секонд6",
          label_en: "Second",
          color: "#4E6D76",
        },
      ],
    },
  });

  // В строке лежит "Second" — это slug. По value ("Секонд67") искать нельзя:
  // там подпись на базовом языке, а не значение.
  const option = field.options.get("Second");

  expect(option?.value).toBe("Second");
  expect(field.options.get("Секонд67")).toBeUndefined();
  expect(localized(option!.labels, "cyr", option!.label)).toBe("Секонд6");
  expect(localized(option!.labels, "en", option!.label)).toBe("Second");
});

test("STATUS: варианты собираются из трёх списков по стадиям", () => {
  const field = toField({
    id: "1",
    slug: "status123",
    type: "STATUS",
    attributes: {
      has_color: true,
      // Тот самый ключ, которого нет у STATUS: старый читатель смотрел
      // только сюда и поэтому не находил вариантов вовсе.
      options: [],
      todo: { options: [{ value: "todo", label_en: "Test todo", color: "#27AE60" }] },
      progress: { options: [{ value: "test_in_progress", label_en: "In progress" }] },
      complete: { options: [{ value: "complete slug", label_en: "Done" }] },
    },
  });

  expect([...field.options.keys()]).toEqual(["todo", "test_in_progress", "complete slug"]);
  expect(field.options.get("todo")?.group).toBe("todo");
  expect(field.options.get("complete slug")?.group).toBe("complete");
});

test("у варианта STATUS нет базовой подписи — показываем значение", () => {
  const field = toField({
    id: "1",
    type: "STATUS",
    attributes: { todo: { options: [{ value: "todo", label_cyr: "Режа" }] } },
  });

  const option = field.options.get("todo")!;

  // Языка данных "en" в этом варианте нет, label тоже нет — остаётся value.
  expect(localized(option.labels, "en", option.label || option.value)).toBe("todo");
  expect(localized(option.labels, "cyr", option.label || option.value)).toBe("Режа");
});

test("label_to_* — подпись обратной стороны связи, а не язык", () => {
  const field = toField({
    id: "1",
    slug: "listings_id",
    type: "LOOKUP",
    attributes: { label: "Listings", label_en: "Listings", label_to_en: "example_hey" },
  });

  expect(field.labels).toEqual({ en: "Listings" });
});

test("у типов без вариантов индекс пустой", () => {
  expect(toField({ id: "1", type: "SINGLE_LINE", attributes: { options: [] } }).options.size).toBe(0);
});

test("подпись обычного поля берётся из колонки label", () => {
  expect(toField({ id: "1", slug: "title", label: "Заголовок" }).label).toBe("Заголовок");
});

test("у связи подпись берётся из attributes: в колонке служебное имя", () => {
  const field = toField({
    id: "1",
    slug: "listings_id",
    type: "LOOKUP",
    label: "FROM example_hey TO listings",
    attributes: { label: "Listings" },
  });

  expect(field.label).toBe("Listings");
});

test("связь без имени в attributes показывает слаг, а не служебное имя", () => {
  const field = toField({
    id: "1",
    slug: "role_id",
    type: "LOOKUP",
    label: "FROM users TO role",
    attributes: {},
  });

  expect(field.label).toBe("role_id");
});

test("id связи приходит под двумя именами и разворачивается в одно", () => {
  expect(toField({ id: "1", relation_field: "r" }).relationId).toBe("r");
  expect(toField({ id: "1", relation_id: "r" }).relationId).toBe("r");
  expect(toField({ id: "1" }).relationId).toBeNull();
});

test("view_fields ненастроенной связи отбрасываются", () => {
  // Бэкенд кладёт туда само поле-связь исходной таблицы — по нему
  // в данных связанной строки ничего не найдётся.
  const relation = toRelation(
    {
      id: "r",
      type: "Many2One",
      table_from: { id: "from", slug: "example_hey" },
      table_to: { id: "to", slug: "listings" },
      view_fields: [
        { id: "f1", slug: "listings_id", table_id: "from" },
        { id: "f2", slug: "title", table_id: "to" },
      ],
    },
    "example_hey",
  );

  expect(relation.viewFieldSlugs).toEqual(["title"]);
  expect(relation.toSlug).toBe("listings");
});

test("связь читается с той стороны, с которой на неё смотрят", () => {
  /*
   * Одна и та же строка в базе: from = example_hey, to = listings.
   * Для listings целевая таблица — example_hey, и поля показа берутся
   * оттуда же. Раньше toSlug всегда возвращал table_to, то есть саму
   * таблицу, из которой смотрим.
   */
  const dto = {
    id: "r",
    type: "Many2One",
    table_from: { id: "from", slug: "example_hey" },
    table_to: { id: "to", slug: "listings" },
    view_fields: [
      { id: "f1", slug: "name", table_id: "from" },
      { id: "f2", slug: "title", table_id: "to" },
    ],
  };

  const relation = toRelation(dto, "listings");

  expect(relation.toSlug).toBe("example_hey");
  expect(relation.viewFieldSlugs).toEqual(["name"]);
});

test("колонка-связь отдаётся только той таблице, в которой она лежит", () => {
  // field_from — колонка в table_from. Для listings её в своей строке
  // нет, и автозаполнению из этой связи брать нечего.
  const dto = {
    id: "r",
    type: "Many2One",
    table_from: { id: "from", slug: "example_hey" },
    table_to: { id: "to", slug: "listings" },
    field_from: "listings_id",
  };

  expect(toRelation(dto, "example_hey").fieldFrom).toBe("listings_id");
  expect(toRelation(dto, "listings").fieldFrom).toBe("");
});

/*
 * Сторона и колонка-ссылка: по ним вкладка карточки отбирает связанные
 * строки. Имя колонки берётся из ответа, а не собирается из слагов —
 * у второй связи на ту же таблицу оно другое, и собранное промахивается.
 */
test("связь знает свою сторону и настоящее имя колонки-ссылки", () => {
  const dto = {
    id: "r",
    type: "Many2One",
    table_from: { id: "from", slug: "order" },
    table_to: { id: "to", slug: "customer" },
    field_from: "customer_id_2",
  };

  // Мы — table_to: на нас ссылаются чужие строки.
  expect(toRelation(dto, "customer")).toMatchObject({
    direction: "incoming",
    linkField: "customer_id_2",
    toSlug: "order",
  });

  // Мы — table_from: колонка-ссылка в нашей строке.
  expect(toRelation(dto, "order")).toMatchObject({
    direction: "outgoing",
    linkField: "customer_id_2",
    toSlug: "customer",
  });
});

test("своё имя колонки перебивает field_from, пустое — правило бэкенда", () => {
  const dto = {
    id: "r",
    type: "Many2Dynamic",
    table_from: { id: "from", slug: "task" },
    table_to: { id: "to", slug: "customer" },
    field_from: "customer_id",
    relation_field_slug: "owner_id",
  };

  expect(toRelation(dto, "customer").linkField).toBe("owner_id");
  // Ни того, ни другого — остаётся имя, которым колонку завёл бэкенд.
  expect(toRelation({ ...dto, field_from: "", relation_field_slug: "" }, "customer").linkField).toBe(
    "customer_id",
  );
});
