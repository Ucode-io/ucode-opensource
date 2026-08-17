import { expect, test } from "vitest";
import { toSchema } from "./schema";

/*
 * Безымянная колонка-связь называется своим слагом — `orders_id`.
 * Он про устройство базы, а не про то, что в колонке; человеку нужна
 * подпись чужой таблицы. Случай не редкий: обратную сторону связи
 * бэкенд заводит сам и без имени вовсе.
 */
const relation = {
  id: "r1",
  type: "Many2One",
  table_from: { id: "t1", slug: "example_hey" },
  table_to: { id: "t2", slug: "orders", label: "Заказы", attributes: { label_cyr: "Буюртма" } },
  field_from: "orders_id",
};

const lookup = (attributes: Record<string, unknown> = {}) => ({
  id: "f1",
  slug: "orders_id",
  type: "LOOKUP",
  relation_field: "r1",
  attributes,
});

test("безымянной связи имя даёт чужая таблица — и на её языках", () => {
  const schema = toSchema({ fields: [lookup()] }, { relations: [relation] }, "example_hey");

  expect(schema.fields[0]?.label).toBe("Заказы");
  expect(schema.fields[0]?.labels).toEqual({ cyr: "Буюртма" });
});

test("заданное админом имя не перебивается", () => {
  const schema = toSchema(
    { fields: [lookup({ label: "Мои заказы" })] },
    { relations: [relation] },
    "example_hey",
  );

  expect(schema.fields[0]?.label).toBe("Мои заказы");
});

test("без подписи у чужой таблицы остаётся слаг колонки", () => {
  const bare = { ...relation, table_to: { id: "t2", slug: "orders" } };
  const schema = toSchema({ fields: [lookup()] }, { relations: [bare] }, "example_hey");

  // Слаг таблицы — тоже не идеал, но он хотя бы есть; выдумывать нечего.
  expect(schema.fields[0]?.label).toBe("orders");
});

test("обычное поле не трогается", () => {
  const schema = toSchema(
    { fields: [{ id: "f2", slug: "price", type: "NUMBER", label: "Цена" }] },
    { relations: [] },
    "example_hey",
  );

  expect(schema.fields[0]?.label).toBe("Цена");
});
