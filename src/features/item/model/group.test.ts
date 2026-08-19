import { expect, test } from "vitest";
import { groupEntries, groupKey, visibleEntries } from "./group";

test("заголовок вставляется на каждой смене значения", () => {
  const rows = [
    { guid: "1", status: "todo" },
    { guid: "2", status: "todo" },
    { guid: "3", status: "done" },
  ];

  const entries = groupEntries(rows, "status");

  expect(entries).toEqual([
    { kind: "header", key: '"todo"', row: 0, count: 2 },
    { kind: "row", key: '"todo"', row: 0 },
    { kind: "row", key: '"todo"', row: 1 },
    { kind: "header", key: '"done"', row: 2, count: 1 },
    { kind: "row", key: '"done"', row: 2 },
  ]);
});

test("пустое значение — своя группа, а не слипание с нулём и false", () => {
  expect(groupKey(null)).toBe(groupKey(undefined));
  expect(groupKey(null)).not.toBe(groupKey(0));
  expect(groupKey(null)).not.toBe(groupKey(false));
  expect(groupKey(null)).not.toBe(groupKey(""));
});

// String() склеил бы ["a","b"] и ["a,b"] в одну группу.
test("списки MULTISELECT различаются честно", () => {
  expect(groupKey(["a", "b"])).not.toBe(groupKey(["a,b"]));
});

test("свёрнутая группа прячет строки, заголовок остаётся", () => {
  const rows = [
    { guid: "1", status: "todo" },
    { guid: "2", status: "done" },
  ];

  const shown = visibleEntries(groupEntries(rows, "status"), new Set(['"todo"']));

  expect(shown).toEqual([
    { kind: "header", key: '"todo"', row: 0, count: 1 },
    { kind: "header", key: '"done"', row: 1, count: 1 },
    { kind: "row", key: '"done"', row: 1 },
  ]);
});
