import { expect, test } from "vitest";
import { isValidSlug, slugify } from "./slug";

test("кириллица транслитерируется, а не выбрасывается", () => {
  // Иначе у русского названия слаг пустой — поле нельзя создать.
  expect(slugify("Название")).toBe("nazvanie");
  expect(slugify("Дата рождения")).toBe("data_rozhdeniya");
  expect(slugify("Ўзбекча ном")).toBe("ozbekcha_nom");
});

test("слаг годится как имя колонки", () => {
  expect(slugify("  Total, $  ")).toBe("total");
  expect(slugify("2 категория")).toBe("f_2_kategoriya");
  expect(slugify("!!!")).toBe("");
  expect(isValidSlug(slugify("Дата рождения"))).toBe(true);
});

test("зарезервированные слова экранируются", () => {
  // ALTER TABLE ... ADD COLUMN order — синтаксическая ошибка postgres.
  expect(slugify("Order")).toBe("order_");
  expect(slugify("Группа")).toBe("gruppa");
  expect(isValidSlug("order")).toBe(false);
  expect(isValidSlug("Order")).toBe(false);
  expect(isValidSlug("order_")).toBe(true);
});
