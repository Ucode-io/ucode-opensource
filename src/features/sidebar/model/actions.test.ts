import { expect, test } from "vitest";
import { actionsFor, typeWordKey } from "./actions";
import { showsTable } from "./types";
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

test("действия без своего экрана не показываются никому", () => {
  // «Настройки пункта» остались в типах и переводах, но обработчика
  // у них нет: кнопка, которая ничего не делает, хуже отсутствующей.
  // Право и роль тут ни при чём — их нет ни у кого.
  expect(ids(node({ type: "FOLDER", kind: "group" }), true)).not.toContain("settings");
});

test("шаблон делают из папки и только администратор", () => {
  const folder = node({ type: "FOLDER", kind: "group" });

  expect(ids(folder, true)).toContain("make-template");
  // Не администратору шаблоны недоступны — так было и в старой админке.
  expect(ids(folder, false)).not.toContain("make-template");
  // У таблицы шаблонить нечего: в тело уезжает дерево пункта.
  expect(ids(node({ type: "TABLE" }), true)).not.toContain("make-template");
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

test("ссылка на существующую таблицу открывается таблицей, а не объяснением", () => {
  // Старая админка привязывала готовую таблицу пунктом типа LINK
  // с table_id (LinkTableModal.jsx:53). Адреса наружу у него нет,
  // а таблица и её view — есть.
  expect(showsTable({ type: "LINK", tableId: "t1" })).toBe(true);
  expect(showsTable({ type: "LINK", tableId: "" })).toBe(false);
  expect(showsTable({ type: "TABLE", tableId: "" })).toBe(true);
  expect(showsTable({ type: "WIKI", tableId: "t1" })).toBe(false);
});
