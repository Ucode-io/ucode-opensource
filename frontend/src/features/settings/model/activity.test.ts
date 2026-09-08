import { expect, test } from "vitest";
import { unwrapEntry } from "./activity";

test("конверт data снимается, объект раскладывается по строкам", () => {
  expect(unwrapEntry('{"data":{"name":"Склад"}}')).toBe('{\n  "name": "Склад"\n}');
});

test("объект без конверта читается так же", () => {
  expect(unwrapEntry('{"name":"Склад"}')).toBe('{\n  "name": "Склад"\n}');
});

/* Шлюз подставляет пустую карту вместо отсутствующего значения. */
test("пустая карта — это отсутствие значения, а не пустой объект", () => {
  expect(unwrapEntry('{"data":{}}')).toBe("");
  expect(unwrapEntry("")).toBe("");
});

test("текст ошибки не JSON — показывается как есть", () => {
  expect(unwrapEntry("rpc error: code = Unavailable")).toBe("rpc error: code = Unavailable");
  expect(unwrapEntry('{"data":"not found"}')).toBe("not found");
});
