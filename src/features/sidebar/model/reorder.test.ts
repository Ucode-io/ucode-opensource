import { expect, test } from "vitest";
import { planMove } from "./reorder";
import type { MenuNode } from "./types";

const node = (id: string, over: Partial<MenuNode> = {}): MenuNode => ({
  id,
  label: id,
  labels: {},
  icon: "",
  type: "TABLE",
  kind: "leaf",
  tableId: "",
  folder: "",
  embedUrl: "",
  microfrontendId: "",
  params: {},
  order: 0,
  isStatic: false,
  parentId: "root",
  children: [],
  raw: {},
  can: { read: true, write: true, update: true, delete: true, settings: true },
  ...over,
});

const folder = (id: string) => node(id, { type: "FOLDER", kind: "group" });

const a = node("a");
const b = node("b");
const c = node("c");
const f = folder("f");
const level = [a, b, c, f];

const drop = (target: MenuNode, position: "before" | "after" | "inside", extra = {}) => ({
  target,
  position,
  targetSiblings: level,
  targetParentId: "root",
  targetPath: ["root"],
  ...extra,
});

test("перестановка внутри уровня", () => {
  const plan = planMove({ node: c, parentId: "root" }, drop(a, "before"));

  expect(plan?.siblings.map((n) => n.id)).toEqual(["c", "a", "b", "f"]);
  expect(plan?.movedId).toBeUndefined();
});

test("вставка после соседа", () => {
  const plan = planMove({ node: a, parentId: "root" }, drop(b, "after"));

  expect(plan?.siblings.map((n) => n.id)).toEqual(["b", "a", "c", "f"]);
});

test("порядок не изменился — плана нет, запрос не уйдёт", () => {
  // b уже стоит сразу после a.
  expect(planMove({ node: b, parentId: "root" }, drop(a, "after"))).toBeNull();
});

test("перенос в раскрытую папку встаёт в конец её детей", () => {
  const x = node("x", { parentId: "f" });
  const plan = planMove({ node: a, parentId: "root" }, drop(f, "inside", { insideSiblings: [x] }));

  expect(plan?.parentId).toBe("f");
  expect(plan?.siblings.map((n) => n.id)).toEqual(["x", "a"]);
  expect(plan?.movedId).toBe("a");
});

test("перенос в нераскрытую папку не требует её содержимого", () => {
  const plan = planMove({ node: a, parentId: "root" }, drop(f, "inside"));

  expect(plan?.parentId).toBe("f");
  expect(plan?.siblings.map((n) => n.id)).toEqual(["a"]);
});

test("внутрь обычного пункта класть нельзя", () => {
  expect(planMove({ node: a, parentId: "root" }, drop(b, "inside"))).toBeNull();
});

test("перетаскивание на себя ничего не даёт", () => {
  expect(planMove({ node: a, parentId: "root" }, drop(a, "before"))).toBeNull();
});

test("папку нельзя уронить внутрь собственного потомка", () => {
  // f лежит в корне, inner — внутри f. Тащим f в inner: получилось бы кольцо.
  const inner = folder("inner");

  expect(
    planMove(
      { node: f, parentId: "root" },
      {
        target: inner,
        position: "inside",
        targetSiblings: [inner],
        targetParentId: "f",
        targetPath: ["root", "f"],
      },
    ),
  ).toBeNull();
});

test("папку нельзя поставить соседом собственного потомка", () => {
  const inner = node("inner", { parentId: "f" });

  expect(
    planMove(
      { node: f, parentId: "root" },
      {
        target: inner,
        position: "before",
        targetSiblings: [inner],
        targetParentId: "f",
        targetPath: ["root", "f"],
      },
    ),
  ).toBeNull();
});

test("приход из другой папки помечается сменой родителя", () => {
  const outsider = node("z", { parentId: "f" });
  const plan = planMove({ node: outsider, parentId: "f" }, drop(b, "before"));

  expect(plan?.parentId).toBe("root");
  expect(plan?.siblings.map((n) => n.id)).toEqual(["a", "z", "b", "c", "f"]);
  expect(plan?.movedId).toBe("z");
});
