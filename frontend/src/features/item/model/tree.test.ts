import { expect, test } from "vitest";
import { flattenTree, groupByParent } from "./tree";

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

test("плоский список собирается в дерево по колонке родителя", () => {
  const rows = [
    { guid: "a", order_id: null },
    { guid: "a1", order_id: "a" },
    { guid: "a2", order_id: "a" },
    { guid: "b", order_id: null },
  ];

  const childrenOf = groupByParent(rows, "order_id");

  expect(childrenOf.get("")?.map((row) => row.guid)).toEqual(["a", "b"]);
  expect(childrenOf.get("a")?.map((row) => row.guid)).toEqual(["a1", "a2"]);

  // has_child в таких строках нет — признак виден по самой карте.
  const { meta } = flattenTree(childrenOf, new Set(["a"]));
  expect(meta.get("a")?.hasChild).toBe(true);
  expect(meta.get("a1")?.hasChild).toBe(false);
});

test("сирота встаёт в корень, а не пропадает", () => {
  // Родителя «z» в наборе нет: он не связан с открытой записью.
  const rows = [
    { guid: "a", order_id: null },
    { guid: "lost", order_id: "z" },
  ];

  const childrenOf = groupByParent(rows, "order_id");

  expect(childrenOf.get("")?.map((row) => row.guid)).toEqual(["a", "lost"]);
  expect(childrenOf.has("z")).toBe(false);
});
