import { describe, expect, it } from "vitest";
import type { Item } from "./types";
import {
  DEFAULT_SORT,
  compareKeys,
  formatPivotSort,
  isHidden,
  parsePivotSort,
  pathKey,
  pivotKey,
  pivotTable,
  toAggregation,
  type PivotSetup,
} from "./pivot";

const rows: Item[] = [
  { guid: "1", status: "todo", team: "a", amount: 10 },
  { guid: "2", status: "todo", team: "b", amount: 30 },
  { guid: "3", status: "done", team: "a", amount: 20 },
  { guid: "4", status: "done", team: "a", amount: null },
];

const setup = (over: Partial<PivotSetup> = {}): PivotSetup => ({
  rowSlugs: ["status"],
  colSlug: "",
  valueSlug: "",
  aggregation: "count",
  sort: DEFAULT_SORT,
  skipEmpty: false,
  ...over,
});

const at = (table: ReturnType<typeof pivotTable>, key: string) =>
  table.rows.find((row) => row.key === key);

describe("pivotTable", () => {
  it("считает количество строк по клеткам", () => {
    const table = pivotTable(rows, setup({ colSlug: "team" }));

    expect(table.rows.map((row) => row.key)).toEqual(["done", "todo"]);
    expect(table.columns).toEqual(["a", "b"]);
    expect(at(table, "todo")?.cells.get("a")).toBe(1);
    expect(at(table, "done")?.cells.get("a")).toBe(2);
    expect(table.total).toBe(4);
  });

  it("строка без числа не попадает в сумму, но и не считается нулём", () => {
    // Сумма по пустому значению — это «складывать нечего», а не 0:
    // иначе среднее по колонке поехало бы вниз на каждой пустой строке.
    const table = pivotTable(rows, setup({ valueSlug: "amount", aggregation: "avg" }));

    expect(at(table, "done")?.total).toBe(20);
    expect(table.count).toBe(3);
  });

  it("сумма по строкам и колонкам сходится с общим итогом", () => {
    const table = pivotTable(
      rows,
      setup({ colSlug: "team", valueSlug: "amount", aggregation: "sum" }),
    );

    expect(at(table, "todo")?.total).toBe(40);
    expect(table.columnTotals.get("a")).toBe(30);
    expect(table.total).toBe(60);
  });

  it("без поля строк сводить нечего", () => {
    const table = pivotTable(rows, setup({ rowSlugs: [], colSlug: "team" }));

    expect(table.rows).toEqual([]);
    expect(table.count).toBe(0);
  });

  it("min и max берутся по значениям, а не по строкам", () => {
    const base = setup({ valueSlug: "amount" });

    expect(pivotTable(rows, { ...base, aggregation: "min" }).total).toBe(10);
    expect(pivotTable(rows, { ...base, aggregation: "max" }).total).toBe(30);
  });

  it("уровни вкладываются, а подытог считается по поддереву", () => {
    const table = pivotTable(rows, setup({ rowSlugs: ["status", "team"] }));

    expect(table.rows.map((row) => [row.key, row.level])).toEqual([
      ["done", 0],
      ["a", 1],
      ["todo", 0],
      ["a", 1],
      ["b", 1],
    ]);

    const done = at(table, "done");
    expect(done?.total).toBe(2);
    expect(done?.hasChildren).toBe(true);
    expect(table.rows.find((row) => row.level === 1 && row.key === "b")?.total).toBe(1);
  });

  it("подытог группы — свод по поддереву, а не сумма подытогов детей", () => {
    // Для min это разные числа: минимум минимумов совпал бы случайно,
    // а среднее средних не совпадает почти никогда.
    const table = pivotTable(
      rows,
      setup({ rowSlugs: ["status", "team"], valueSlug: "amount", aggregation: "avg" }),
    );

    // todo: 10 и 30 → 20, а не среднее из «a=10» и «b=30» по числу групп.
    expect(at(table, "todo")?.total).toBe(20);
  });

  it("сортировка по итогу и по колонке", () => {
    const byTotal = pivotTable(
      rows,
      setup({ valueSlug: "amount", aggregation: "sum", sort: { by: { kind: "total" }, desc: true } }),
    );
    expect(byTotal.rows.map((row) => row.key)).toEqual(["todo", "done"]);

    const byColumn = pivotTable(
      rows,
      setup({
        colSlug: "team",
        sort: { by: { kind: "column", key: "b" }, desc: true },
      }),
    );
    // У done в колонке «b» клетки нет вовсе — при убывании она уходит вниз.
    expect(byColumn.rows.map((row) => row.key)).toEqual(["todo", "done"]);
  });

  it("колонки отсортированы, а не в порядке появления строк", () => {
    const shuffled: Item[] = [
      { guid: "1", status: "todo", team: "z" },
      { guid: "2", status: "todo", team: "a" },
    ];

    expect(pivotTable(shuffled, setup({ colSlug: "team" })).columns).toEqual(["a", "z"]);
  });
});

describe("pivotKey", () => {
  it("пустое значение — своя клетка, а не пропуск", () => {
    expect(pivotKey(null)).toBe("");
    expect(pivotKey(undefined)).toBe("");
  });

  it("список склеивается целиком: строка не должна попасть в две клетки", () => {
    // Иначе сумма по колонке перестала бы сходиться с общим итогом.
    expect(pivotKey(["a", "b"])).toBe("a, b");
  });
});

describe("compareKeys", () => {
  it("числа сравниваются как числа, а не как текст", () => {
    expect(["10", "9", "100"].sort(compareKeys)).toEqual(["9", "10", "100"]);
  });

  it("«пусто» последнее: это не значение, а его отсутствие", () => {
    expect(["b", "", "a"].sort(compareKeys)).toEqual(["a", "b", ""]);
  });
});

describe("порядок строк", () => {
  const withEmpty: Item[] = [
    { guid: "1", status: "todo" },
    { guid: "2", status: null },
    { guid: "3", status: "done" },
  ];

  it("«пусто» остаётся в хвосте и при обратной сортировке", () => {
    // Разворот применяется после проверки на «пусто»: иначе отсутствие
    // значения оказывалось бы первой строкой сводной.
    const keys = (desc: boolean) =>
      pivotTable(withEmpty, setup({ sort: { by: { kind: "label" }, desc } })).rows.map(
        (row) => row.key,
      );

    expect(keys(false)).toEqual(["done", "todo", ""]);
    expect(keys(true)).toEqual(["todo", "done", ""]);
  });
});

describe("сортировка в адресе", () => {
  it("ключ колонки не путается с итогом", () => {
    // Колонка со значением «total» — обычное дело: у неё своя приставка.
    const sort = parsePivotSort(formatPivotSort({ by: { kind: "column", key: "total" }, desc: true }));

    expect(sort).toEqual({ by: { kind: "column", key: "total" }, desc: true });
    expect(parsePivotSort("-total")).toEqual({ by: { kind: "total" }, desc: true });
  });

  it("непонятное значение — сортировка по подписи", () => {
    expect(parsePivotSort(undefined)).toEqual(DEFAULT_SORT);
    expect(parsePivotSort("что-то")).toEqual(DEFAULT_SORT);
  });
});

describe("isHidden", () => {
  it("свёрнутая группа прячет потомков, но не себя", () => {
    const collapsed = new Set([pathKey(["done"])]);

    expect(isHidden(["done"], collapsed)).toBe(false);
    expect(isHidden(["done", "a"], collapsed)).toBe(true);
    expect(isHidden(["todo", "a"], collapsed)).toBe(false);
  });

  it("значение с пробелом не выдаёт себя за путь из двух уровней", () => {
    // Отсюда разделитель, которого в значениях полей быть не может.
    const collapsed = new Set([pathKey(["Москва Иванов"])]);

    expect(isHidden(["Москва", "Иванов"], collapsed)).toBe(false);
  });
});

describe("toAggregation", () => {
  it("непонятное значение — количество: оно считается по любому полю", () => {
    expect(toAggregation(undefined)).toBe("count");
    expect(toAggregation("median")).toBe("count");
    expect(toAggregation("sum")).toBe("sum");
  });
});

describe("без пустых", () => {
  const withEmpty: Item[] = [
    { guid: "1", status: "todo", team: "a" },
    { guid: "2", status: null, team: "a" },
    { guid: "3", status: "todo", team: null },
  ];

  it("пустые не прячутся, а не считаются — иначе итог не сойдётся", () => {
    const table = pivotTable(withEmpty, setup({ colSlug: "team", skipEmpty: true }));

    expect(table.rows.map((row) => row.key)).toEqual(["todo"]);
    expect(table.columns).toEqual(["a"]);
    // Счётчик показывает цену: из трёх строк в расчёт вошла одна.
    expect(table.count).toBe(1);
    expect(table.total).toBe(1);
  });

  it("без поля колонок пустой ключ колонки ничего не отсеивает", () => {
    // Иначе «без пустых» опустошало бы сводную целиком: ключ колонки
    // пуст у всех строк, пока поле колонок не выбрано.
    const table = pivotTable(withEmpty, setup({ skipEmpty: true }));

    expect(table.count).toBe(2);
  });
});
