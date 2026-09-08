import { expect, test } from "vitest";
import { columnWindow } from "./column-window";

test("пустой список от виртуализатора показывает все колонки", () => {
  expect(columnWindow(30, 2, [])).toEqual({ pinned: 2, from: 2, to: 30, before: 0, after: 0 });
});

test("окно посередине заменяет края распорками", () => {
  expect(columnWindow(30, 0, [10, 11, 12])).toEqual({
    pinned: 0,
    from: 10,
    to: 13,
    before: 10,
    after: 17,
  });
});

/*
 * Закреплённые колонки рисуются всегда и отдельно. Виртуализатор про
 * это не знает и отдаёт их индексы наравне с прочими — окно обязано
 * начаться после них, иначе те же ячейки окажутся в строке дважды.
 */
test("закреплённые колонки в окно не попадают", () => {
  expect(columnWindow(30, 3, [0, 1, 2, 3, 4])).toEqual({
    pinned: 3,
    from: 3,
    to: 5,
    before: 0,
    after: 25,
  });
});

test("прокрутка до конца оставляет распорку только слева", () => {
  expect(columnWindow(30, 0, [27, 28, 29])).toEqual({
    pinned: 0,
    from: 27,
    to: 30,
    before: 27,
    after: 0,
  });
});

/*
 * Колонку могли убрать между тем, как виртуализатор посчитал окно, и тем,
 * как строка отрисовалась. Распорка отрицательной ширины сломала бы
 * colSpan, поэтому окно обрезается по числу колонок.
 */
test("окно за пределами списка обрезается", () => {
  expect(columnWindow(5, 0, [8, 9])).toEqual({ pinned: 0, from: 5, to: 5, before: 5, after: 0 });
});

/*
 * Главное свойство: закреплённые, окно и обе распорки вместе покрывают
 * ровно все колонки. Разойдись счёт на одну — ячейки строки перестанут
 * попадать в свои колонки, и вся таблица поедет вбок.
 */
test("ряд покрывает все колонки при любом окне", () => {
  const total = 12;
  for (let pinned = 0; pinned <= 3; pinned++) {
    for (let start = 0; start < total; start++) {
      const visible = [start, start + 1, start + 2].filter((index) => index < total);
      const shown = columnWindow(total, pinned, visible);
      expect(shown.pinned + shown.before + (shown.to - shown.from) + shown.after).toBe(total);
    }
  }
});
