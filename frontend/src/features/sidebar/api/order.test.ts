import { expect, test } from "vitest";
import { toNodes } from "./menus";

/** Пункт виден, только если есть право чтения — поэтому оно во всех примерах. */
const readable = { permission: { read: true } };

test("порядок ответа сервера сохраняется, а не переставляется по алфавиту", () => {
  // Раньше клиент сортировал по label — и «Абрикос» всплывал наверх,
  // хотя сервер поставил его последним.
  const nodes = toNodes(
    [
      { id: "1", label: "Яблоко", type: "TABLE", data: readable },
      { id: "2", label: "Банан", type: "TABLE", data: readable },
      { id: "3", label: "Абрикос", type: "TABLE", data: readable },
    ],
    "ru",
  );

  expect(nodes.map((n) => n.label)).toEqual(["Яблоко", "Банан", "Абрикос"]);
});

test("пункт без id выпадает, но порядок остальных не сдвигается на него", () => {
  const nodes = toNodes(
    [
      { label: "Битый", type: "TABLE", data: readable },
      { id: "2", label: "Первый", type: "TABLE", data: readable },
    ],
    "ru",
  );

  expect(nodes.map((n) => n.label)).toEqual(["Первый"]);
  expect(nodes[0]!.order).toBe(0);
});

test("без права чтения пункт не показывается", () => {
  // Правило старой версии: data.permission.read ложный — пункта нет.
  // Показать пункт, который вернёт отказ, хуже, чем не показать.
  const nodes = toNodes(
    [
      { id: "1", label: "Видимый", type: "TABLE", data: { permission: { read: true } } },
      { id: "2", label: "Скрытый", type: "TABLE", data: { permission: { read: false } } },
      { id: "3", label: "Без прав вообще", type: "TABLE" },
    ],
    "ru",
  );

  expect(nodes.map((n) => n.label)).toEqual(["Видимый"]);
});

test("нумерация не сбивается после скрытых пунктов", () => {
  const nodes = toNodes(
    [
      { id: "1", label: "Скрытый", type: "TABLE", data: { permission: { read: false } } },
      { id: "2", label: "Первый", type: "TABLE", data: { permission: { read: true } } },
      { id: "3", label: "Второй", type: "TABLE", data: { permission: { read: true } } },
    ],
    "ru",
  );

  // order — позиция в ответе сервера, а не в отфильтрованном списке:
  // перетаскивание считает соседей по видимому списку, а порядок
  // на сервере задаётся индексом в отправляемом массиве.
  expect(nodes.map((n) => [n.label, n.order])).toEqual([
    ["Первый", 1],
    ["Второй", 2],
  ]);
});

test("системные пункты не показываются в дереве меню", () => {
  // Настройки, Файлы и Пользователи бэкенд заводит в каждом проекте,
  // но их экраны живут отдельно. Старая версия прятала их так же.
  const nodes = toNodes(
    [
      { id: "c57eedc3-a954-4262-a0af-376c65b5a280", label: "Settings", type: "FOLDER", data: readable },
      { id: "8a6f913a-e3d4-4b73-9fc0-c942f343d0b9", label: "Files", type: "FOLDER", data: readable },
      { id: "9e988322-cffd-484c-9ed6-460d8701551b", label: "Users", type: "FOLDER", data: readable },
      { id: "own", label: "Заказы", type: "TABLE", data: readable },
    ],
    "ru",
  );

  expect(nodes.map((n) => n.label)).toEqual(["Заказы"]);
});
