import { expect, test } from "vitest";
import { groupEntries, groupKey, visibleEntries } from "./group";

const key = (...values: unknown[]) => groupKey(values);

test("заголовок вставляется на каждой смене значения", () => {
  const rows = [
    { guid: "1", status: "todo" },
    { guid: "2", status: "todo" },
    { guid: "3", status: "done" },
  ];

  const entries = groupEntries(rows, ["status"]);

  expect(entries).toEqual([
    { kind: "header", key: key("todo"), path: [key("todo")], level: 0, row: 0, count: 2 },
    { kind: "row", path: [key("todo")], row: 0 },
    { kind: "row", path: [key("todo")], row: 1 },
    { kind: "header", key: key("done"), path: [key("done")], level: 0, row: 2, count: 1 },
    { kind: "row", path: [key("done")], row: 2 },
  ]);
});

test("второй уровень вкладывается в первый", () => {
  const rows = [
    { guid: "1", team: "a", status: "todo" },
    { guid: "2", team: "a", status: "done" },
    { guid: "3", team: "b", status: "todo" },
  ];

  const entries = groupEntries(rows, ["team", "status"]);

  expect(entries.filter((entry) => entry.kind === "header").map((entry) => entry.level)).toEqual([
    0, 1, 1, 0, 1,
  ]);
  // Счёт верхнего уровня — все его строки, а не строки первой подгруппы.
  expect(entries.find((entry) => entry.kind === "header")?.count).toBe(2);
});

test("одинаковое значение в разных родителях — разные группы", () => {
  // Иначе «todo» команды A и «todo» команды B слиплись бы в одну группу,
  // и свёртка одной прятала бы обе.
  const rows = [
    { guid: "1", team: "a", status: "todo" },
    { guid: "2", team: "b", status: "todo" },
  ];

  const headers = groupEntries(rows, ["team", "status"]).filter(
    (entry) => entry.kind === "header" && entry.level === 1,
  );

  expect(headers).toHaveLength(2);
  expect(headers[0]).not.toEqual(headers[1]);
});

test("пустое значение — своя группа, а не слипание с нулём и false", () => {
  expect(key(null)).toBe(key(undefined));
  expect(key(null)).not.toBe(key(0));
  expect(key(null)).not.toBe(key(false));
  expect(key(null)).not.toBe(key(""));
});

// String() склеил бы ["a","b"] и ["a,b"] в одну группу.
test("списки MULTISELECT различаются честно", () => {
  expect(key(["a", "b"])).not.toBe(key(["a,b"]));
});

test("свёрнутая группа прячет строки, заголовок остаётся", () => {
  const rows = [
    { guid: "1", status: "todo" },
    { guid: "2", status: "done" },
  ];

  const shown = visibleEntries(groupEntries(rows, ["status"]), new Set([key("todo")]));

  expect(shown).toEqual([
    { kind: "header", key: key("todo"), path: [key("todo")], level: 0, row: 0, count: 1 },
    { kind: "header", key: key("done"), path: [key("done")], level: 0, row: 1, count: 1 },
    { kind: "row", path: [key("done")], row: 1 },
  ]);
});

test("свёрнутая группа прячет и вложенные заголовки", () => {
  const rows = [
    { guid: "1", team: "a", status: "todo" },
    { guid: "2", team: "b", status: "done" },
  ];

  const shown = visibleEntries(groupEntries(rows, ["team", "status"]), new Set([key("a")]));

  // От свёрнутой команды остаётся только она сама.
  expect(shown.map((entry) => entry.kind)).toEqual(["header", "header", "header", "row"]);
  expect(shown[1]).toMatchObject({ kind: "header", level: 0 });
});
