import { expect, test } from "vitest";
import { toFile } from "./files";

test("расширение берётся из имени скачивания, а не из заголовка", () => {
  // В title лежит то, что человек напечатал; расширение — часть файла.
  const file = toFile({ title: "Договор", file_name_download: "contract.final.PDF" });

  expect(file.extension).toBe("PDF");
  expect(file.title).toBe("Договор");
});

test("без имени и заголовка карточка подписана id, а не пустотой", () => {
  expect(toFile({ id: "7f3" }).title).toBe("7f3");
  expect(toFile({ id: "7f3" }).extension).toBe("");
});

test("размер приходит и числом, и строкой — обе ветки шлюза", () => {
  expect(toFile({ file_size: 2048 }).size).toBe(2048);
  expect(toFile({ file_size: "2048" }).size).toBe(2048);
  expect(toFile({}).size).toBe(0);
});

test("точка в начале имени не делает всё имя расширением", () => {
  // «.gitignore» — файл без расширения, а не расширение GITIGNORE.
  expect(toFile({ file_name_download: ".gitignore" }).extension).toBe("");
});
