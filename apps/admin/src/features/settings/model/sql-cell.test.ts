import { expect, test } from "vitest";
import { sqlCell } from "./sql-cell";

test("null отличается от строки со словом null", () => {
  expect(sqlCell(null)).toEqual({ text: "null", kind: "null" });
  expect(sqlCell("null")).toEqual({ text: "null", kind: "text" });
});

test("числа и булевы — как есть, ноль и false не теряются", () => {
  expect(sqlCell(0)).toEqual({ text: "0", kind: "number" });
  expect(sqlCell(false)).toEqual({ text: "false", kind: "boolean" });
});

/* json/jsonb и массивы postgres приезжают вложенным значением. */
test("вложенное значение сворачивается в JSON одной строкой", () => {
  expect(sqlCell({ tags: ["a", "b"] })).toEqual({ text: '{"tags":["a","b"]}', kind: "text" });
  expect(sqlCell([1, 2]).text).toBe("[1,2]");
});

/* NULL в числовой колонке остаётся NULL, а не нулём и не текстом. */
test("null из любой колонки красится как null, а не как её тип", () => {
  expect(sqlCell(null).kind).toBe("null");
});

/* uuid и дата — только по типу колонки: значение в JS у обоих строка. */
test("uuid и дата различаются по типу колонки, не по форме строки", () => {
  expect(sqlCell("e7b1a2c0-1111-4a2b-9c3d-000000000000", "uuid").kind).toBe("uuid");
  expect(sqlCell("2026-09-14T03:45:22Z", "timestamptz").kind).toBe("date");
  expect(sqlCell("2026-09-14T03:45:22Z", "timestamp").kind).toBe("date");
  expect(sqlCell("2026-09-14", "date").kind).toBe("date");
});

/* Похожая на uuid строка в обычной text-колонке — просто текст. */
test("без объявленного типа колонки строка остаётся текстом, даже похожая на uuid", () => {
  expect(sqlCell("e7b1a2c0-1111-4a2b-9c3d-000000000000").kind).toBe("text");
  expect(sqlCell("e7b1a2c0-1111-4a2b-9c3d-000000000000", "text").kind).toBe("text");
});

/* NULL остаётся NULL и в uuid/date-колонке — правило типа JS сильнее. */
test("null в uuid- или date-колонке не подменяется её типом", () => {
  expect(sqlCell(null, "uuid").kind).toBe("null");
  expect(sqlCell(null, "timestamptz").kind).toBe("null");
});
