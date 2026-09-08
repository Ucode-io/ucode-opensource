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
  required: false,
  validation: null,
  editable: true,
    locked: false,
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

test("у EMAIL проверка есть и без настройки, но своя её перебивает", () => {
  const email = field({ type: "EMAIL" });

  expect(cellError(email, "a@b.uz")).toBe(null);
  // Своего сообщения нет — покажет общее, его подставляет тот, кто рисует.
  expect(cellError(email, "qwe")).toEqual({ kind: "pattern", message: "" });
  // Пустое проверяет required, а не выражение.
  expect(cellError(email, "")).toBe(null);

  // Настройка админа главнее встроенной: под неё написаны живые данные.
  const own = field({ type: "EMAIL", validation: { pattern: /^\d+$/, message: "Только цифры" } });
  expect(cellError(own, "a@b.uz")).toEqual({ kind: "pattern", message: "Только цифры" });
  expect(cellError(own, "12")).toBe(null);
});

test("новая строка проверяется целиком, но только по правимым колонкам", () => {
  const columns = [
    field({ id: "1", slug: "name", required: true }),
    field({ id: "2", slug: "email", validation: { pattern: /^\S+@\S+$/, message: "" } }),
    // Значение считает бэкенд — спрашивать его с человека нечего.
    field({ id: "3", slug: "number", type: "INCREMENT_ID", required: true }),
    // Право отняла РОЛЬ: заполнить такую колонку нечем и при заведении.
    field({ id: "4", slug: "denied", required: true, editable: false, locked: true }),
  ];

  expect([...rowErrors(columns, {}).keys()]).toEqual(["name"]);
  expect([...rowErrors(columns, { name: "Аня", email: "нет собаки" }).keys()]).toEqual(["email"]);
  expect(rowErrors(columns, { name: "Аня", email: "a@b.uz" }).size).toBe(0);
});

/*
 * «Только чтение» админа — про правку заведённой записи, а не про её
 * заполнение: иначе обязательное поле, закрытое от правки, нельзя было
 * бы задать вообще нигде. Так же считает и старая админка (`&& isEditing`).
 */
test("поле «только чтение» при заведении заполняется и потому обязательно", () => {
  const readOnly = field({ id: "5", slug: "contract", required: true, editable: false });

  expect([...rowErrors([readOnly], {}).keys()]).toEqual(["contract"]);
  expect(rowErrors([readOnly], { contract: "Д-17" }).size).toBe(0);
});

test("списки и объекты выражением не проверяются", () => {
  const any = field({ validation: { pattern: /^\d+$/, message: "" } });

  expect(cellError(any, ["a", "b"])).toBe(null);
  expect(cellError(any, { lat: 1 })).toBe(null);
  expect(cellError(any, 42)).toBe(null);
  expect(cellError(any, "42a")).toEqual({ kind: "pattern", message: "" });
});

/*
 * Пробелы — не значение. В старой админке это проверяется вручную
 * и только у MULTI_LINE; у нас — у любой строки, потому что причина
 * одна и та же.
 */
test("строка из пробелов не заполняет обязательное поле", () => {
  const required = field({ required: true });

  expect(cellError(required, "   ")).toEqual({ kind: "required", message: "" });
  expect(cellError(required, "\n\t")).toEqual({ kind: "required", message: "" });
  expect(cellError(required, " Аня ")).toBe(null);
});
