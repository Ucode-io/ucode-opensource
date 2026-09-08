import { expect, test } from "vitest";
import { fileKind, fileName } from "./file-kind";

test("uuid-приставка бэкенда в имя не попадает", () => {
  expect(fileName("https://cdn/media/3f1c9b0e-2a4d-4c6b-9f1a-2e5d7c8b9a01_Отчёт.pdf")).toBe(
    "Отчёт.pdf",
  );
  // Подчёркивания в самом имени остаются: режем по uuid, а не по первому «_».
  expect(fileName("https://cdn/my_media/2024_итоги_года.xlsx")).toBe("2024_итоги_года.xlsx");
  // Приставка и ничего кроме неё — показывать нечего, оставляем как есть.
  expect(fileName("https://cdn/media/3f1c9b0e-2a4d-4c6b-9f1a-2e5d7c8b9a01_")).toBe(
    "3f1c9b0e-2a4d-4c6b-9f1a-2e5d7c8b9a01_",
  );
});

test("вид файла определяется по расширению, приставка ему не мешает", () => {
  expect(fileKind("https://cdn/media/3f1c9b0e-2a4d-4c6b-9f1a-2e5d7c8b9a01_фото.PNG")).toBe("image");
  expect(fileKind("https://cdn/media/отчёт.pdf?v=2")).toBe("pdf");
  expect(fileKind("https://cdn/media/файл-без-расширения")).toBe("other");
});
