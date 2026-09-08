import { expect, test } from "vitest";
import type { Field } from "@/features/table";
import { filterKind, operatorsFor, rangeBound } from "./filter-kind";

const field = (type: string): Field =>
  ({ type, options: new Map() }) as unknown as Field;

test("флажок фильтруется, число — своим видом, а не текстом", () => {
  expect(filterKind(field("CHECKBOX"))).toBe("boolean");

  // «Содержит» по числовой колонке бэкенд собирает как равенство,
  // а на буквах отвечает 500 — поэтому у чисел его нет вовсе.
  for (const type of ["NUMBER", "FLOAT", "FLOAT_NOLIMIT"]) {
    expect(filterKind(field(type))).toBe("number");
  }
  expect(operatorsFor("number")).not.toContain("contains");
});

test("текстовый фильтр есть у всего, что лежит в VARCHAR", () => {
  // Колонка обычная строка — регулярка `~*` по ней работает, а значит
  // и «содержит» (FIELD-AUDIT, F6). Штрихкод и время сюда же: искать
  // товар по коду на складе — первое, что делают.
  for (const type of ["LINK", "CODE", "MANUAL_STRING", "RANDOM_TEXT", "TIME", "CODABAR"]) {
    expect(filterKind(field(type))).toBe("text");
  }

  // А это не VARCHAR или не отбирается по смыслу: массив (`= ANY`
  // вместо «содержит»), хэш пароля, адрес файла.
  for (const type of ["LOOKUPS", "MULTI_FILE", "PASSWORD", "PHOTO", "INCREMENT_NUMBER"]) {
    expect(filterKind(field(type))).toBeNull();
  }
});

test("верхняя граница времени дотягивается до конца суток", () => {
  const time = field("DATE_TIME");

  // «с 6 по 10»: нижняя — полночь 6-го, верхняя — конец 10-го.
  expect(rangeBound(time, "between", 0, "2026-01-06")).toBe("2026-01-06");
  expect(rangeBound(time, "between", 1, "2026-01-10")).toBe("2026-01-10T23:59:59.999");

  // «после 10-го» — строго после его конца; «до 10-го» — строго до начала.
  expect(rangeBound(time, "after", 0, "2026-01-10")).toBe("2026-01-10T23:59:59.999");
  expect(rangeBound(time, "before", 0, "2026-01-10")).toBe("2026-01-10");
});

test("у календарной даты и у числа границу не трогаем", () => {
  // DATE сравнивается днями, а не моментами: там час только мешает.
  expect(rangeBound(field("DATE"), "between", 1, "2026-01-10")).toBe("2026-01-10");
  expect(rangeBound(field("NUMBER"), "between", 1, "42")).toBe("42");
  expect(rangeBound(field("DATE_TIME"), "between", 1, "")).toBe("");
});
