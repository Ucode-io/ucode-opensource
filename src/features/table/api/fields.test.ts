import { expect, test } from "vitest";
import { EMPTY_DRAFT, toDraft } from "../model/field-draft";
import { toField } from "./normalize";
import { toCreateBody } from "./fields";

const AT = { tableSlug: "bookings", language: "en", id: "11111111-2222-3333-4444-555555555555" };

test("подпись уходит и колонкой, и на языке данных", () => {
  const body = toCreateBody({ ...EMPTY_DRAFT, label: " Дата брони ", slug: "data_broni" }, {
    ...AT,
    language: "cyr",
  });

  expect(body).toMatchObject({
    id: AT.id,
    slug: "data_broni",
    label: "Дата брони",
    // Ключ называется table_id, но это слаг ТАБЛИЦЫ, а не поля: на чужой
    // слаг ручка отвечает 500 «not found».
    table_id: "bookings",
  });
  expect(body.attributes).toMatchObject({ label_cyr: "Дата брони" });
});

test("варианты мультиселекта: в строку ляжет slug, подпись останется в value", () => {
  const body = toCreateBody(
    {
      ...EMPTY_DRAFT,
      label: "Метки",
      slug: "metki",
      type: "MULTISELECT",
      options: [
        { label: "Срочно", color: "red" },
        { label: "", color: "gray" },
      ],
    },
    AT,
  );

  const attributes = body.attributes as { options: Record<string, unknown>[]; has_color: boolean };

  // Пустая строка — не вариант, а незаполненная строка формы.
  expect(attributes.options).toHaveLength(1);
  expect(attributes.options[0]).toMatchObject({
    slug: "srochno",
    value: "Срочно",
    label: "Срочно",
    label_en: "Срочно",
  });
  expect(attributes.has_color).toBe(true);
});

test("варианты статуса разложены по стадиям", () => {
  const body = toCreateBody(
    {
      ...EMPTY_DRAFT,
      label: "Статус",
      slug: "status",
      type: "STATUS",
      groups: {
        todo: [{ label: "Новая", color: "gray" }],
        progress: [{ label: "В работе", color: "blue" }],
        complete: [],
      },
    },
    AT,
  );

  const attributes = body.attributes as Record<string, { options: Record<string, unknown>[] }>;

  expect(attributes.todo!.options[0]).toMatchObject({ value: "novaya", label_en: "Новая" });
  expect(attributes.progress!.options[0]).toMatchObject({ value: "v_rabote" });
  expect(attributes.complete!.options).toEqual([]);
});

test("созданное поле читается собственным разбором", () => {
  // Круг замкнут: то, что мы записали, мы же и покажем в ячейке.
  const body = toCreateBody(
    {
      ...EMPTY_DRAFT,
      label: "Метки",
      slug: "metki",
      type: "MULTISELECT",
      options: [{ label: "Срочно", color: "red" }],
    },
    AT,
  );

  const field = toField({
    slug: "metki",
    label: "Метки",
    type: "MULTISELECT",
    attributes: body.attributes as Record<string, unknown>,
  });

  expect(field.options.get("srochno")).toMatchObject({ value: "srochno", label: "Срочно" });
  expect(field.hasColor).toBe(true);
});

test("настройки агрегата уезжают под теми именами, под которыми их читает бэкенд", () => {
  const body = toCreateBody(
    {
      ...EMPTY_DRAFT,
      label: "Сумма заказов",
      slug: "orders_sum",
      type: "FORMULA",
      aggregate: {
        type: "SUMM",
        // Слаг и id связи одной строкой — так это лежит в attributes.
        tableFrom: "orders#rel-1",
        field: "total",
        rounds: "2",
        filters: [
          { key: "paid#SWITCH#", value: true },
          // Незаполненное условие отбросит ничего, поэтому не отправляем.
          { key: "", value: null },
        ],
      },
    },
    AT,
  );

  expect(body.attributes).toMatchObject({
    type: "SUMM",
    table_from: "orders#rel-1",
    sum_field: "total",
    number_of_rounds: 2,
    formula_filters: [{ key: "paid#SWITCH#", value: true }],
  });
});

test("агрегат читается обратно ровно тем же черновиком", () => {
  const body = toCreateBody(
    {
      ...EMPTY_DRAFT,
      label: "Сумма",
      slug: "summa",
      type: "FORMULA",
      aggregate: {
        type: "AVG",
        tableFrom: "orders#rel-1",
        field: "total",
        rounds: "",
        filters: [{ key: "tags#MULTISELECT#", value: ["hot"] }],
      },
    },
    AT,
  );

  const field = toField({
    slug: "summa",
    type: "FORMULA",
    attributes: body.attributes as Record<string, unknown>,
  });

  expect(toDraft(field, "en").aggregate).toEqual({
    type: "AVG",
    tableFrom: "orders#rel-1",
    field: "total",
    // Округление не задавали — ключа в attributes нет, и обратно
    // приходит пустая строка, а не «0».
    rounds: "",
    filters: [{ key: "tags#MULTISELECT#", value: ["hot"] }],
  });
});

test("выражение frontend-формулы уезжает и читается как есть", () => {
  const body = toCreateBody(
    { ...EMPTY_DRAFT, label: "Итого", slug: "itogo", type: "FORMULA_FRONTEND", formula: " price * count " },
    AT,
  );

  expect(body.attributes).toMatchObject({ formula: "price * count" });
  // Ключи агрегата у него не появляются: чужая настройка в attributes
  // читалась бы следующим как своя.
  expect(body.attributes).not.toHaveProperty("sum_field");
});

test("автозаполнение и мультиязычность уезжают колонками, а не в attributes", () => {
  const draft = {
    ...EMPTY_DRAFT,
    label: "Телефон клиента",
    slug: "client_phone",
    type: "SINGLE_LINE",
    autofillTable: "clients#client_id",
    autofillField: "phone",
    automatic: true,
    multilanguage: true,
  };

  const body = toCreateBody(draft, AT);

  expect(body).toMatchObject({
    autofill_table: "clients#client_id",
    autofill_field: "phone",
    automatic: true,
    enable_multilanguage: true,
  });
  // В attributes их класть нельзя: бэкенд оттуда настройку не читает —
  // она сохранится и не заработает.
  expect(body.attributes).not.toHaveProperty("autofill_table");
  expect(body.attributes).not.toHaveProperty("enable_multilanguage");
});

test("мультиязычность не уезжает у типа, который её не умеет", () => {
  // Флаг поставили текстовому полю, потом сменили тип: шлюз разводит
  // по колонке на язык только SINGLE_LINE и MULTI_LINE, у остальных
  // в базе остался бы висеть признак без единого языкового поля.
  const body = toCreateBody(
    { ...EMPTY_DRAFT, label: "Цена", slug: "price", type: "NUMBER", multilanguage: true },
    AT,
  );

  expect(body.enable_multilanguage).toBe(false);
});

test("настройки-колонки читаются из ответа, а не из эха в attributes", () => {
  /*
   * Бэкенд копирует те же имена в attributes, отдавая layout
   * (layout.go), и после правки эхо разъезжается с колонками. Истина —
   * колонка: её пишет и читает PUT.
   */
  const field = toField({
    slug: "client_phone",
    type: "SINGLE_LINE",
    autofill_table: "clients#client_id",
    autofill_field: "phone",
    automatic: true,
    enable_multilanguage: true,
    attributes: {
      autofill_table: "stale#stale_id",
      autofill_field: "stale",
      automatic: false,
      enable_multilanguage: false,
    },
  } as Parameters<typeof toField>[0]);

  expect(toDraft(field, "en")).toMatchObject({
    autofillTable: "clients#client_id",
    autofillField: "phone",
    automatic: true,
    multilanguage: true,
  });
});

test("кнопка уезжает иконкой и функцией и читается обратно", () => {
  const body = toCreateBody(
    {
      ...EMPTY_DRAFT,
      label: "Отправить",
      slug: "send",
      type: "BUTTON",
      icon: " tabler:send ",
      functionId: "fn-1",
    },
    AT,
  );

  expect(body.attributes).toMatchObject({ icon: "tabler:send", function: "fn-1" });

  const field = toField({
    slug: "send",
    type: "BUTTON",
    attributes: body.attributes as Record<string, unknown>,
  });

  expect(toDraft(field, "en")).toMatchObject({ icon: "tabler:send", functionId: "fn-1" });
});

test("функция не уезжает у поля, которое перестало быть кнопкой", () => {
  // Иначе в attributes осталась бы мёртвая настройка, и следующий
  // читатель принял бы её за действующую.
  const body = toCreateBody(
    { ...EMPTY_DRAFT, label: "Имя", slug: "name", icon: "tabler:send", functionId: "fn-1" },
    AT,
  );

  expect(body.attributes).not.toHaveProperty("function");
  expect(body.attributes).not.toHaveProperty("icon");
});
