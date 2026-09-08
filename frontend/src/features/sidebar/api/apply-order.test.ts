import { expect, test } from "vitest";
import { applyOrder, removeFromLevel } from "./menus";
import type { MenuNode } from "../model/types";

const node = (id: string): MenuNode => ({
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
});

test("уровень переставляется в заданном порядке", () => {
  const cached = { menus: [{ id: "a", label: "A" }, { id: "b", label: "B" }, { id: "c", label: "C" }] };

  const next = applyOrder(cached, [node("c"), node("a"), node("b")], undefined);

  expect(next.menus?.map((m) => m.id)).toEqual(["c", "a", "b"]);
  // Подписи не теряются: берём сырые пункты из кэша, а не пересобираем их.
  expect(next.menus?.map((m) => m.label)).toEqual(["C", "A", "B"]);
});

test("пункт из другой папки добавляется в уровень", () => {
  const cached = { menus: [{ id: "a", label: "A" }] };
  const moved = { id: "z", label: "Z" };

  const next = applyOrder(cached, [node("z"), node("a")], moved);

  expect(next.menus?.map((m) => m.id)).toEqual(["z", "a"]);
});

test("пункт, которого нет ни в кэше, ни в переносе, не создаёт дыру", () => {
  // Иначе в списке появился бы undefined и уровень падал бы при рендере.
  const cached = { menus: [{ id: "a", label: "A" }] };

  const next = applyOrder(cached, [node("a"), node("ghost")], undefined);

  expect(next.menus?.map((m) => m.id)).toEqual(["a"]);
  expect(next.count).toBe(1);
});

test("пустой кэш не роняет перестановку", () => {
  expect(applyOrder(undefined, [node("a")], undefined).menus).toEqual([]);
});

test("при переносе в другую папку пункт убирается с исходного уровня", () => {
  // Иначе он на мгновение виден в обоих местах сразу.
  const source = { menus: [{ id: "a" }, { id: "z" }, { id: "b" }] };

  expect(removeFromLevel(source, "z").menus?.map((m) => m.id)).toEqual(["a", "b"]);
  expect(removeFromLevel(source, "z").count).toBe(2);
});

test("удаление несуществующего пункта не меняет уровень", () => {
  const source = { menus: [{ id: "a" }] };

  expect(removeFromLevel(source, "ghost").menus?.map((m) => m.id)).toEqual(["a"]);
});
