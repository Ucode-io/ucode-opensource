import { expect, test } from "vitest";
import { GAP, pageCount, pageItems } from "./pagination";

test("страниц мало — рисуем все, без пропусков", () => {
  expect(pageItems(1, 7)).toEqual([1, 2, 3, 4, 5, 6, 7]);
});

test("начало списка: пропуск только справа", () => {
  expect(pageItems(1, 16)).toEqual([1, 2, 3, 4, 5, GAP, 16]);
});

test("середина: пропуски с обеих сторон", () => {
  expect(pageItems(8, 16)).toEqual([1, GAP, 7, 8, 9, GAP, 16]);
});

test("конец списка: пропуск только слева", () => {
  expect(pageItems(16, 16)).toEqual([1, GAP, 12, 13, 14, 15, 16]);
});

test("многоточие не появляется ради одной спрятанной страницы", () => {
  // «1 … 3» заняло бы столько же места, сколько «1 2 3», но скрыло бы
  // переход. Проверяем обе границы, где окно отрывается от края.
  expect(pageItems(4, 16)).toEqual([1, 2, 3, 4, 5, GAP, 16]);
  expect(pageItems(5, 16)).toEqual([1, GAP, 4, 5, 6, GAP, 16]);
  expect(pageItems(13, 16)).toEqual([1, GAP, 12, 13, 14, 15, 16]);
  expect(pageItems(12, 16)).toEqual([1, GAP, 11, 12, 13, GAP, 16]);
});

test("ширина полосы не меняется при переходах", () => {
  const widths = new Set(
    Array.from({ length: 16 }, (_, index) => pageItems(index + 1, 16).length),
  );

  expect([...widths]).toEqual([7]);
});

test("страница вне диапазона прижимается к краю", () => {
  expect(pageItems(0, 16)).toEqual(pageItems(1, 16));
  expect(pageItems(99, 16)).toEqual(pageItems(16, 16));
});

test("пустой список — одна страница, а не ноль", () => {
  expect(pageCount(0, 50)).toBe(1);
  expect(pageItems(1, pageCount(0, 50))).toEqual([1]);
});

test("счёт страниц округляется вверх", () => {
  expect(pageCount(51, 50)).toBe(2);
  expect(pageCount(100, 50)).toBe(2);
});
