import { expect, test } from "vitest";
import type { Field } from "@/features/table";
import { isFieldVisible } from "./visibility";

const field = (attributes: Record<string, unknown>) => ({ attributes }) as Field;

test("без правила поле показывается всегда", () => {
  expect(isFieldVisible(field({}), { status: "done" })).toBe(true);
  // Ожидаемое значение без поля, за которым следить, — не правило.
  expect(isFieldVisible(field({ hide_path: "done" }), { status: "done" })).toBe(true);
  // Записи ещё нет: прятать по отсутствующим значениям нечего.
  expect(isFieldVisible(field({ hide_path_field: "status", hide_path: "done" }), undefined)).toBe(
    true,
  );
});

test("равенство сравнивает строковый вид, а не тип", () => {
  const rule = field({ hide_path_field: "status", hide_path: "done" });

  expect(isFieldVisible(rule, { status: "done" })).toBe(true);
  expect(isFieldVisible(rule, { status: "todo" })).toBe(false);
  // Пустое и отсутствующее значение условию не отвечают.
  expect(isFieldVisible(rule, { status: "" })).toBe(false);
  expect(isFieldVisible(rule, {})).toBe(false);

  /*
   * Ожидаемое человек набирает строкой, а в записи лежит число или
   * булево. На `===` такое правило не срабатывало никогда — это и есть
   * поведение старой админки, которое здесь исправлено.
   */
  expect(isFieldVisible(field({ hide_path_field: "count", hide_path: "5" }), { count: 5 })).toBe(
    true,
  );
  expect(isFieldVisible(field({ hide_path_field: "paid", hide_path: "true" }), { paid: true })).toBe(
    true,
  );
});

test("набор сравнивается как множество: порядок вариантов не условие", () => {
  const rule = field({ hide_path_field: "tags", hide_path: ["a", "b"] });

  expect(isFieldVisible(rule, { tags: ["a", "b"] })).toBe(true);
  expect(isFieldVisible(rule, { tags: ["b", "a"] })).toBe(true);
  expect(isFieldVisible(rule, { tags: ["a"] })).toBe(false);
  expect(isFieldVisible(rule, { tags: ["a", "b", "c"] })).toBe(false);

  // Один вариант, записанный строкой, и он же массивом — одно правило.
  expect(isFieldVisible(field({ hide_path_field: "tags", hide_path: "a" }), { tags: ["a"] })).toBe(
    true,
  );
});

test("числовые границы исключающие: min — больше, max — меньше", () => {
  const min = field({ hide_path_field: "count", hide_path: "10", type: "min" });
  const max = field({ hide_path_field: "count", hide_path: "10", type: "max" });

  expect(isFieldVisible(min, { count: 11 })).toBe(true);
  expect(isFieldVisible(min, { count: 10 })).toBe(false);
  expect(isFieldVisible(max, { count: 9 })).toBe(true);
  expect(isFieldVisible(max, { count: 10 })).toBe(false);

  // Не число с обеих сторон — условию отвечать нечем.
  expect(isFieldVisible(min, { count: "много" })).toBe(false);
  expect(isFieldVisible(min, {})).toBe(false);
});

test("чужой type сравнивается равенством, а не как граница", () => {
  /*
   * Тот же ключ у поля FORMULA хранит вид агрегата. Ветка чисел
   * включается ровно на двух строках, поэтому «SUMM» уходит в обычное
   * сравнение и агрегат не превращается в условие видимости.
   */
  const rule = field({ hide_path_field: "status", hide_path: "done", type: "SUMM" });

  expect(isFieldVisible(rule, { status: "done" })).toBe(true);
  expect(isFieldVisible(rule, { status: "todo" })).toBe(false);
});
