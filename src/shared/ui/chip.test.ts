import { expect, test } from "vitest";
import { CHIP_HEX, hexToChipColor } from "./chip";

test("палитра последнего конструктора ложится на токены", () => {
  // Четыре именованных цвета, которые предлагает актуальная форма поля.
  expect(hexToChipColor("#EAECF0")).toBe("gray");
  expect(hexToChipColor("#FFD6AE")).toBe("orange");
  expect(hexToChipColor("#AAF0C4")).toBe("green");
  expect(hexToChipColor("#FECDCA")).toBe("red");
});

test("произвольные цвета из накопленных данных читаются как намерение", () => {
  expect(hexToChipColor("#00D717")).toBe("green");
  expect(hexToChipColor("#27AE60")).toBe("green");
  expect(hexToChipColor("#F31D2F")).toBe("red");
});

test("почти чёрное — серое, а не синее", () => {
  // Оттенок у #181D21 формально синий, но глазу это чёрный,
  // и на тёмном фоне он неразличим.
  expect(hexToChipColor("#181D21")).toBe("gray");
});

test("тёмный оранжевый — коричневый", () => {
  expect(hexToChipColor("#7A4A18")).toBe("brown");
});

test("мусор вместо цвета не роняет ячейку", () => {
  expect(hexToChipColor("")).toBe("gray");
  expect(hexToChipColor("не цвет")).toBe("gray");
  expect(hexToChipColor("#zzz")).toBe("gray");
});

test("короткая и длинная запись дают один результат", () => {
  expect(hexToChipColor("#0f0")).toBe("green");
  expect(hexToChipColor("#00ff00")).toBe("green");
  expect(hexToChipColor("#00ff00ff")).toBe("green");
});

test("цвет варианта читается тем же оттенком, каким записан", () => {
  // Иначе созданный здесь вариант тут же меняет цвет при чтении.
  for (const [name, hex] of Object.entries(CHIP_HEX)) {
    expect(hexToChipColor(hex)).toBe(name);
  }
});
