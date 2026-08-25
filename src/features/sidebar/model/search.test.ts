import { expect, test } from "vitest";
import { matchMenus, type MenuMatch } from "./search";
import type { MenuNode } from "./types";

const node = (label: string, labels: Record<string, string> = {}): MenuNode => ({
  id: label,
  label,
  labels,
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

const item = (label: string, labels?: Record<string, string>): MenuMatch => ({
  node: node(label, labels),
  trail: [],
});

const labels = (matches: MenuMatch[]) => matches.map((match) => match.node.label);

test("пустой запрос ничего не находит", () => {
  // Не «находит всё»: список из всего дерева вместо меню — не результат.
  expect(matchMenus([item("Заявки")], "   ")).toEqual([]);
});

test("совпадения с начала имени идут первыми", () => {
  const found = labels(matchMenus([item("Обработка заявок"), item("Заявки")], "за"));

  expect(found).toEqual(["Заявки", "Обработка заявок"]);
});

test("ищем и в подписях на других языках проекта", () => {
  // Человек помнит пункт под тем именем, которое видел, а видел он его
  // на своём языке интерфейса.
  const found = matchMenus([item("Заявки", { en: "Orders", ru: "Заявки" })], "ord");

  expect(labels(found)).toEqual(["Заявки"]);
});
