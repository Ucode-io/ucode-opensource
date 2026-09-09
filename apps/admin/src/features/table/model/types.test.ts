import { expect, test } from "vitest";
import { optionOf } from "./types";
import type { Field, FieldOption } from "./types";

const option = (over: Partial<FieldOption>): FieldOption => ({
  value: "",
  label: "",
  labels: {},
  color: null,
  icon: null,
  group: null,
  ...over,
});

const field = (options: FieldOption[]): Field =>
  ({
    options: new Map(options.map((item) => [item.value, item])),
  }) as Field;

test("вариант ищется по значению строки, а у старых полей — по подписи", () => {
  const status = field([
    option({ value: "novaya_zayavka", label: "Новая заявка", color: "#8B5CF6" }),
    option({ value: "nedozvon", label: "Недозвон", labels: { cyr: "Қўнғироқ" }, color: "#22A06B" }),
  ]);

  // Обычный случай: в строке лежит то, чем вариант проиндексирован.
  expect(optionOf(status, "novaya_zayavka")?.color).toBe("#8B5CF6");

  /*
   * Поле прежнего поколения: слаг варианту приписали позже, а в строках
   * осталась подпись. Без этой ветки чип серый и без цвета — то, с чего
   * началась правка.
   */
  expect(optionOf(status, "Новая заявка")?.color).toBe("#8B5CF6");
  expect(optionOf(status, "Қўнғироқ")?.value).toBe("nedozvon");

  // Вариант удалили из настроек, а строки с ним остались.
  expect(optionOf(status, "Отказ")).toBeUndefined();
});
