import { expect, test } from "vitest";
import type { Field } from "@/features/table";
import { boardColumns, boardOrderAt, groupValue, NO_GROUP } from "./board";

const tabs = [
  { id: "todo", label: "К работе" },
  { id: "done", label: "Готово" },
];

const field = (type: string) => ({ type }) as Field;
const columnsOf = (rows: Record<string, unknown>[], extra = {}) =>
  boardColumns({ rows, tabs, slug: "status", unassigned: "Без значения", ...extra });

test("колонки идут в порядке вариантов, пустые остаются на месте", () => {
  const rows = [{ guid: "1", status: "done" }];

  expect(columnsOf(rows)).toEqual([
    { id: "todo", label: "К работе", rows: [] },
    { id: "done", label: "Готово", rows: [rows[0]] },
    { id: NO_GROUP, label: "Без значения", rows: [] },
  ]);
});

test("строка без значения попадает в последнюю колонку", () => {
  const rows = [
    { guid: "1", status: null },
    { guid: "2", status: "" },
    { guid: "3" },
  ];

  const columns = columnsOf(rows);

  expect(columns[columns.length - 1]).toEqual({
    id: NO_GROUP,
    label: "Без значения",
    rows,
  });
});

test("MULTISELECT кладёт карточку сразу в несколько колонок", () => {
  const rows = [{ guid: "1", status: ["todo", "done"] }];
  const columns = columnsOf(rows);

  expect(columns[0]?.rows).toEqual(rows);
  expect(columns[1]?.rows).toEqual(rows);
});

test("значение вне вариантов получает свою колонку, а не пропадает", () => {
  const rows = [{ guid: "1", status: "снятый вариант" }];

  expect(columnsOf(rows).map((column) => column.id)).toEqual([
    "todo",
    "done",
    "снятый вариант",
    NO_GROUP,
  ]);
});

/*
 * Колонка по связи: вариантов у поля нет вовсе, а подпись лежит
 * в самой строке — рядом со ссылкой бэкенд кладёт связанную запись.
 */
test("колонка из данных подписывается по своей же строке", () => {
  const rows = [
    { guid: "1", status: "guid-соли", status_data: { name: "Соли" } },
    { guid: "2", status: "guid-соли", status_data: { name: "Соли" } },
  ];

  const columns = boardColumns({
    rows,
    tabs: [],
    slug: "status",
    unassigned: "Без значения",
    labelOf: (row) => String((row["status_data"] as { name: string }).name),
  });

  expect(columns).toEqual([
    { id: "guid-соли", label: "Соли", rows },
    { id: NO_GROUP, label: "Без значения", rows: [] },
  ]);
});

test("номер позиции — между соседями, а не занятый соседом", () => {
  const rows = [
    { guid: "1", board_order: 1 },
    { guid: "2", board_order: 2 },
    { guid: "3", board_order: 3 },
  ];

  expect(boardOrderAt(rows, 0)).toBe(0);
  expect(boardOrderAt(rows, 1)).toBe(1.5);
  expect(boardOrderAt(rows, 3)).toBe(4);
  expect(boardOrderAt([], 0)).toBe(1);
});

test("карточки без номера считаются по своему месту", () => {
  const rows = [{ guid: "1" }, { guid: "2" }, { guid: "3" }];

  // Места 1, 2, 3 — значит между вторым и третьим будет 2.5.
  expect(boardOrderAt(rows, 2)).toBe(2.5);
});

test("одинаковые номера соседей не дают третьего такого же", () => {
  const rows = [
    { guid: "1", board_order: 2 },
    { guid: "2", board_order: 2 },
  ];

  expect(boardOrderAt(rows, 1)).toBe(2.5);
});

/*
 * Порядок строк задаёт сервер и между запросами он гуляет; колонки
 * от этого меняться местами не должны — иначе доска перетасовывается
 * после каждой правки.
 */
test("колонки из данных стоят по алфавиту, а не в порядке строк", () => {
  const rows = [{ guid: "1", status: "b" }, { guid: "2", status: "a" }];

  expect(boardColumns({ rows, tabs: [], slug: "status", unassigned: "—" }).map((c) => c.id)).toEqual(
    ["a", "b", NO_GROUP],
  );
});

test("бросок в колонку без значения снимает значение, а не пишет пустую строку", () => {
  expect(groupValue(field("STATUS"), NO_GROUP)).toBeNull();
  expect(groupValue(field("MULTISELECT"), NO_GROUP)).toBeNull();
});

test("у MULTISELECT значение уезжает списком", () => {
  expect(groupValue(field("MULTISELECT"), "todo")).toEqual(["todo"]);
  expect(groupValue(field("STATUS"), "todo")).toBe("todo");
  expect(groupValue(field("LOOKUP"), "guid-1")).toBe("guid-1");
});
