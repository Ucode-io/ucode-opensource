import { expect, test } from "vitest";
import { actionsFor, typeWordKey } from "./actions";
import type { MenuNode, MenuPermissions } from "./types";

const all: MenuPermissions = {
  read: true,
  write: true,
  update: true,
  delete: true,
  settings: true,
};

const node = (over: Partial<MenuNode> = {}): MenuNode => ({
  id: "1",
  label: "X",
  icon: "",
  type: "TABLE",
  kind: "leaf",
  order: 0,
  isStatic: false,
  parentId: "root",
  children: [],
  raw: {},
  can: all,
  ...over,
});

const ids = (n: MenuNode, admin = false) => actionsFor(n, admin).map((a) => a.id);

test("создание вложенного доступно только у того, что раскрывается", () => {
  expect(ids(node({ type: "FOLDER", kind: "group" }))).toContain("create-table");
  expect(ids(node())).not.toContain("create-table");
  expect(ids(node())).not.toContain("create-folder");
});

test("создание вложенного идёт по праву write", () => {
  // В menu_permission нет колонки "create" — если спрашивать её,
  // пункты «Создать …» не появятся ни у одной папки.
  const folder = node({ type: "FOLDER", kind: "group", can: { ...all, write: false } });

  expect(ids(folder)).not.toContain("create-table");
  expect(ids(folder)).not.toContain("create-folder");
  expect(ids(folder)).not.toContain("create-link");
});

test("без права действие не показывается", () => {
  const readOnly = node({
    can: { ...all, write: false, update: false, delete: false, settings: false },
  });

  expect(ids(readOnly)).toEqual([]);
});

test("шаблон из папки делает только администратор", () => {
  const folder = node({ type: "FOLDER", kind: "group" });

  expect(ids(folder, false)).not.toContain("make-template");
  expect(ids(folder, true)).toContain("make-template");
});

test("у системного пункта нет удаления", () => {
  // Бэкенд его всё равно не удалит (STATIC_MENU_IDS), кнопка была бы ложью.
  expect(ids(node({ isStatic: true }))).not.toContain("delete");
  expect(ids(node({ isStatic: false }))).toContain("delete");
});

test("удаление отделено разделителем и помечено опасным", () => {
  const remove = actionsFor(node(), false).find((a) => a.id === "delete");

  expect(remove?.separated).toBe(true);
  expect(remove?.danger).toBe(true);
});

test("слово для незнакомого типа не оставляет подпись пустой", () => {
  expect(typeWordKey("TABLE")).toBe("menuType.TABLE");
  expect(typeWordKey("SOMETHING_NEW")).toBe("menuType.UNKNOWN");
});
