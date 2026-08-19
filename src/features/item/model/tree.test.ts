import { expect, test } from "vitest";
import { flattenTree } from "./tree";

const children = new Map([
  ["", [{ guid: "a", has_child: true }, { guid: "b", has_child: false }]],
  ["a", [{ guid: "a1", has_child: false }]],
]);

test("раскрытый узел вставляет детей сразу за собой, глубже на уровень", () => {
  const { rows, meta } = flattenTree(children, new Set(["a"]));

  expect(rows.map((row) => row.guid)).toEqual(["a", "a1", "b"]);
  expect(meta.get("a")).toEqual({ depth: 0, hasChild: true });
  expect(meta.get("a1")).toEqual({ depth: 1, hasChild: false });
});

test("свёрнутый узел детей не показывает, даже загруженных", () => {
  const { rows } = flattenTree(children, new Set());
  expect(rows.map((row) => row.guid)).toEqual(["a", "b"]);
});

test("цикл в данных не вешает сборку", () => {
  const cyclic = new Map([
    ["", [{ guid: "a", has_child: true }]],
    ["a", [{ guid: "b", has_child: true }]],
    ["b", [{ guid: "a", has_child: true }]],
  ]);

  const { rows } = flattenTree(cyclic, new Set(["a", "b"]));
  expect(rows.map((row) => row.guid)).toEqual(["a", "b"]);
});
