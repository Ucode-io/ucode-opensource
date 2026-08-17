import { expect, test } from "vitest";
import type { Field } from "@/features/table";
import { cellError, rowErrors } from "./validate";

const field = (over: Partial<Field> = {}): Field => ({
  id: "f",
  slug: "f",
  label: "f",
  labels: {},
  type: "SINGLE_LINE",
  relationId: null,
  options: new Map(),
  multilanguage: false,
  hasColor: false,
  required: false,
  validation: null,
  editable: true,
  attributes: {},
  raw: {},
  ...over,
});

test("необязательное поле пустым быть может, обязательное — нет", () => {
  expect(cellError(field(), "")).toBe(null);
  expect(cellError(field({ required: true }), "")).toEqual({ kind: "required", message: "" });
  expect(cellError(field({ required: true }), null)).toEqual({ kind: "required", message: "" });
  expect(cellError(field({ required: true }), [])).toEqual({ kind: "required", message: "" });
  expect(cellError(field({ required: true }), 0)).toBe(null); // ноль — это значение
});

test("выражение проверяет только заполненное значение", () => {
  const email = field({
    validation: { pattern: /^\S+@\S+$/, message: "Неверная почта" },
  });

  expect(cellError(email, "a@b.uz")).toBe(null);
  // Пустое проверяет required, а не выражение: иначе необязательное поле
  // с выражением стало бы обязательным.
  expect(cellError(email, "")).toBe(null);
  expect(cellError(email, "нет собаки")).toEqual({ kind: "pattern", message: "Неверная почта" });
});

test("новая строка проверяется целиком, но только по правимым колонкам", () => {
  const columns = [
    field({ id: "1", slug: "name", required: true }),
    field({ id: "2", slug: "email", validation: { pattern: /^\S+@\S+$/, message: "" } }),
    // Значение считает бэкенд — спрашивать его с человека нечего.
    field({ id: "3", slug: "number", type: "INCREMENT_ID", required: true }),
    // Правку запретил админ — заполнить такую колонку тоже нечем.
    field({ id: "4", slug: "locked", required: true, editable: false }),
  ];

  expect([...rowErrors(columns, {}).keys()]).toEqual(["name"]);
  expect([...rowErrors(columns, { name: "Аня", email: "нет собаки" }).keys()]).toEqual(["email"]);
  expect(rowErrors(columns, { name: "Аня", email: "a@b.uz" }).size).toBe(0);
});

test("списки и объекты выражением не проверяются", () => {
  const any = field({ validation: { pattern: /^\d+$/, message: "" } });

  expect(cellError(any, ["a", "b"])).toBe(null);
  expect(cellError(any, { lat: 1 })).toBe(null);
  expect(cellError(any, 42)).toBe(null);
  expect(cellError(any, "42a")).toEqual({ kind: "pattern", message: "" });
});
